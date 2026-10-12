package composer

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Composer is the main class for Composer SDK functionality
type Composer struct {
	SecretKey string
	Config    *ComposerConfig
}

// NewComposer creates a new Composer instance with the given secret key and optional config
//
// Constructor validates and sets up the Composer instance:
// - Validates secret key and config
// - Merges default config
// - Validates API version and log level
// - Sets logger filter level
func NewComposer(secretKey string, config *ComposerConfig) (*Composer, error) {
	// Validate secret key
	if secretKey == "" {
		return nil, &ComposerMissingConfigError{
			ConfigName: "secretApiKey",
			Value:      secretKey,
		}
	}

	// Merge provided config with defaults
	mergedConfig := DefaultConfig()
	if config != nil {
		if config.ApiVersion != "" {
			mergedConfig.ApiVersion = config.ApiVersion
		}
		if config.ApiUrl != "" {
			mergedConfig.ApiUrl = config.ApiUrl
		}
		if config.LogLevel != "" {
			mergedConfig.LogLevel = config.LogLevel
		}
		if config.IsEnabled != nil {
			mergedConfig.IsEnabled = config.IsEnabled
		}
	}

	// Validate API version
	if mergedConfig.ApiVersion != V1 {
		return nil, &ComposerInvalidConfigError{
			ConfigName:    "config.apiVersion",
			Value:         mergedConfig.ApiVersion,
			AllowedValues: []string{string(V1)},
		}
	}

	// Validate log level
	validLogLevels := []ComposerLogLevel{
		ComposerLogLevelTrace,
		ComposerLogLevelInfo,
		ComposerLogLevelError,
		ComposerLogLevelNone,
	}
	isValidLogLevel := false
	for _, level := range validLogLevels {
		if mergedConfig.LogLevel == level {
			isValidLogLevel = true
			break
		}
	}

	if !isValidLogLevel {
		allowedValues := make([]string, len(validLogLevels))
		for i, level := range validLogLevels {
			allowedValues[i] = string(level)
		}
		return nil, &ComposerInvalidConfigError{
			ConfigName:    "config.logLevel",
			Value:         mergedConfig.LogLevel,
			AllowedValues: allowedValues,
		}
	}

	// Set logger filter level
	Logger.FilterLevel = logLevelToSeverity(mergedConfig.LogLevel)

	// Resolved after the logger is configured so the message below respects LogLevel.
	if config != nil && config.RequestTimeout != nil {
		if *config.RequestTimeout > 0 {
			mergedConfig.RequestTimeout = config.RequestTimeout
		} else {
			// A non-positive timeout would fail every request the SDK makes.
			Logger.Error(fmt.Sprintf(
				"config.RequestTimeout must be greater than 0. Ignoring %s and using the %s default.",
				*config.RequestTimeout,
				defaultRequestTimeout,
			), nil).Log()
		}
	}

	return &Composer{
		SecretKey: secretKey,
		Config:    mergedConfig,
	}, nil
}

// Dodgeball is an alias for backward compatibility with prior Dodgeball naming conventions
type Dodgeball = Composer

// NewDodgeball is an alias for backward compatibility with prior Dodgeball naming conventions
var NewDodgeball = NewComposer

func (c *Composer) requestTimeout() time.Duration {
	if c.Config.RequestTimeout != nil && *c.Config.RequestTimeout > 0 {
		return *c.Config.RequestTimeout
	}
	return defaultRequestTimeout
}

// CreateErrorResponse creates a standard error response object
func (c *Composer) CreateErrorResponse(code int, message string) *ComposerCheckpointResponse {
	return &ComposerCheckpointResponse{
		Success: false,
		Errors: []ComposerApiError{
			{Code: code, Message: message},
		},
		Version: V1,
		Verification: &ComposerVerification{
			Id:      "",
			Status:  VerificationStatusFailed,
			Outcome: VerificationOutcomeError,
		},
	}
}

// Event sends a tracking event to the Composer API
func (c *Composer) Event(options *TrackOptions) error {
	if options == nil {
		skipEvent("no event was provided")
		return nil
	}
	if IsNullOrEmpty(options.Event.Type) {
		skipEvent("event.type is missing")
		return nil
	}

	if IsNullOrEmpty(options.SessionId) && IsNullOrEmpty(options.SourceToken) {
		skipEvent("Must provide either a sessionId or sourceToken")
		return nil
	}

	// If SDK is disabled, don't send the event
	if c.Config.IsEnabled != nil && !*c.Config.IsEnabled {
		return nil
	}

	// Ensure event has a timestamp
	if options.Event.EventTime == nil {
		now := time.Now().UnixMilli()
		options.Event.EventTime = &now
	}

	// Send event to API
	url := ConstructApiUrl(c.Config.ApiUrl, string(c.Config.ApiVersion)) + "track"
	headers := ConstructApiHeaders(c.SecretKey, "", options.SourceToken, options.UserId, options.SessionId)

	resp, err := MakeRequest(url, "POST", headers, options.Event, c.requestTimeout())
	if err != nil {
		return err
	}
	// Drain and close the body so the connection can be reused
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	// http.Client sets Request on every real response; MakeRequest's stand-in for a request that
	// never got a response leaves it nil, and MakeRequest has already logged that failure.
	requestFailed := resp.Request == nil
	if !requestFailed && (resp.StatusCode < 200 || resp.StatusCode > 299) {
		Logger.Error(fmt.Sprintf("Composer event was not accepted by the API (HTTP %d).", resp.StatusCode), nil).Log()
	}
	return nil
}

func skipEvent(reason string) {
	Logger.Error("Composer event was not sent: "+reason, nil).Log()
}

// Checkpoint submits a checkpoint to Composer and polls for verification if needed
// Handles timeouts, polling, and error conditions
func (c *Composer) Checkpoint(options *CheckpointOptions) (*ComposerCheckpointResponse, error) {
	// Validate required parameters are present
	if IsNullOrEmpty(options.CheckpointName) {
		return nil, &ComposerMissingParameterError{
			Parameter: "checkpointName",
		}
	}

	if IsNullOrEmpty(options.SessionId) && IsNullOrEmpty(options.SourceToken) {
		return nil, &ComposerMissingParameterError{
			Parameter: "Must provide either a sessionId or sourceToken",
		}
	}

	// If SDK is disabled, return a default approved response for development
	if c.Config.IsEnabled != nil && !*c.Config.IsEnabled {
		return &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:       "COMPOSER_IS_DISABLED",
				Status:   VerificationStatusComplete,
				Outcome:  VerificationOutcomeApproved,
				StepData: make(map[string]interface{}),
			},
		}, nil
	}

	// Determine checkpoint timeout
	checkpointTimeout := BaseCheckpointMaxTimeoutMs
	if options.Options != nil && options.Options.Timeout != nil && *options.Options.Timeout > 0 {
		checkpointTimeout = time.Duration(*options.Options.Timeout) * time.Millisecond
	}

	currentPollingWait := BaseCheckpointPollingWaitMs
	hasNeverPolled := true

	// Prepare options for API call
	requestOptions := &CheckpointResponseOptions{}
	if options.Options != nil {
		requestOptions.Sync = options.Options.Sync
	}
	if requestOptions.Sync == nil {
		defaultSync := false
		requestOptions.Sync = &defaultSync
	}

	var response *ComposerCheckpointResponse
	numFailures := 0
	checkpointStartTime := time.Now()

	// Submit checkpoint to API
	url := ConstructApiUrl(c.Config.ApiUrl, string(c.Config.ApiVersion)) + "checkpoint"
	headers := ConstructApiHeaders(c.SecretKey, options.UseVerificationId, options.SourceToken, options.UserId, options.SessionId)

	event := map[string]interface{}{
		"type": options.CheckpointName,
		"data": options.Event.Data,
	}
	if options.Event.Ip != "" {
		event["ip"] = options.Event.Ip
	}
	requestBody := map[string]interface{}{
		"event":   event,
		"options": requestOptions,
	}

	rawResponse, err := MakeRequest(url, "POST", headers, requestBody, c.requestTimeout())
	if err != nil {
		return c.CreateErrorResponse(500, "Unknown evaluation error"), err
	}

	bodyBytes, err := io.ReadAll(rawResponse.Body)
	rawResponse.Body.Close()
	if err != nil {
		return c.CreateErrorResponse(500, "Unknown evaluation error"), err
	}

	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		// Not logging err or the body: the decoder error can echo part of the payload
		Logger.Error("The Composer API returned a response that is not valid JSON.", nil).Log()
		return c.CreateErrorResponse(500, "Unknown evaluation error"), nil
	}

	// Handle error or unsuccessful response
	if response == nil {
		return c.CreateErrorResponse(500, "Unknown evaluation error"), nil
	} else if !response.Success {
		return response, nil
	}

	// Check if verification is resolved or needs polling
	isResolved := response.Verification == nil || response.Verification.Status != VerificationStatusPending
	var verificationId string
	if response.Verification != nil {
		verificationId = response.Verification.Id
	}

	if Logger.IsTraceEnabled() {
		Logger.Trace(fmt.Sprintf("Initial checkpoint response: %s", RedactSensitiveFields(response))).Log()
	}

	// Poll for verification result if not resolved
	// Exponential backoff with capped retries
	for !isResolved &&
		(time.Since(checkpointStartTime) < checkpointTimeout || hasNeverPolled) &&
		numFailures < MaxFailureCount {

		Sleep(currentPollingWait)
		currentPollingWait = time.Duration(float64(currentPollingWait) * 2)
		if currentPollingWait > MaxPollingWaitMs {
			currentPollingWait = MaxPollingWaitMs
		}
		hasNeverPolled = false

		url = ConstructApiUrl(c.Config.ApiUrl, string(c.Config.ApiVersion)) + "verification/" + verificationId
		headers = ConstructApiHeaders(c.SecretKey, options.UseVerificationId, options.SourceToken, options.UserId, options.SessionId)

		rawResponse, err = MakeRequest(url, "GET", headers, nil, c.requestTimeout())
		if err != nil {
			numFailures++
			continue
		}

		bodyBytes, err = io.ReadAll(rawResponse.Body)
		rawResponse.Body.Close()
		if err != nil {
			numFailures++
			continue
		}

		// Decode into a fresh value: unmarshalling into the previous response would keep any
		// field this poll leaves out, such as a stale PENDING verification
		var pollResponse *ComposerCheckpointResponse
		err = json.Unmarshal(bodyBytes, &pollResponse)
		if err != nil {
			// Not logging err or the body: the decoder error can echo part of the payload
			Logger.Error("The Composer API returned a response that is not valid JSON.", nil).Log()
			numFailures++
			continue
		}
		if pollResponse == nil {
			numFailures++
			continue
		}
		response = pollResponse

		if response.Success {
			if response.Verification == nil {
				numFailures++
			} else {
				isResolved = response.Verification.Status != VerificationStatusPending
				numFailures = 0 // reset failure count on successful response
			}
		} else {
			numFailures++
		}
	}

	// If retries exceeded, return timeout error response
	if numFailures >= MaxFailureCount {
		Logger.Error("Service Unavailable: Maximum retry count exceeded", nil).Log()
		isTimeout := true
		timeoutResponse := &ComposerCheckpointResponse{
			Success: false,
			Version: V1,
			Errors: []ComposerApiError{
				{
					Code:    503,
					Message: "Service Unavailable: Maximum retry count exceeded",
				},
			},
			IsTimeout: &isTimeout,
		}
		return timeoutResponse, nil
	}

	// Return the final response
	if Logger.IsTraceEnabled() {
		Logger.Trace(fmt.Sprintf("Returning response: %s", RedactSensitiveFields(response))).Log()
	}
	return response, nil
}

// IsRunning returns true if the checkpoint is still running (pending or blocked) with no verdict yet
func (c *Composer) IsRunning(checkpointResponse *ComposerCheckpointResponse) bool {
	if checkpointResponse.Success && checkpointResponse.Verification != nil &&
		checkpointResponse.Verification.Outcome == VerificationOutcomePending {
		switch checkpointResponse.Verification.Status {
		case VerificationStatusPending, VerificationStatusBlocked:
			return true
		default:
			return false
		}
	}
	return false
}

// IsAllowed returns true if the checkpoint was approved
func (c *Composer) IsAllowed(checkpointResponse *ComposerCheckpointResponse) bool {
	return checkpointResponse.Success &&
		checkpointResponse.Verification != nil &&
		checkpointResponse.Verification.Status == VerificationStatusComplete &&
		checkpointResponse.Verification.Outcome == VerificationOutcomeApproved
}

// IsDenied returns true if the checkpoint was denied
func (c *Composer) IsDenied(checkpointResponse *ComposerCheckpointResponse) bool {
	if checkpointResponse.Success && checkpointResponse.Verification != nil {
		switch checkpointResponse.Verification.Outcome {
		case VerificationOutcomeDenied:
			return true
		default:
			return false
		}
	}
	return false
}

// IsUndecided returns true if the checkpoint is complete but outcome is still pending
func (c *Composer) IsUndecided(checkpointResponse *ComposerCheckpointResponse) bool {
	return checkpointResponse.Success &&
		checkpointResponse.Verification != nil &&
		checkpointResponse.Verification.Status == VerificationStatusComplete &&
		checkpointResponse.Verification.Outcome == VerificationOutcomePending
}

// HasError returns true if the checkpoint response indicates an error
func (c *Composer) HasError(checkpointResponse *ComposerCheckpointResponse) bool {
	return !checkpointResponse.Success ||
		(checkpointResponse.Verification != nil &&
			checkpointResponse.Verification.Outcome == VerificationOutcomeError)
}

// IsTimeout returns true if the checkpoint response timed out
func (c *Composer) IsTimeout(checkpointResponse *ComposerCheckpointResponse) bool {
	return !checkpointResponse.Success && checkpointResponse.IsTimeout != nil && *checkpointResponse.IsTimeout
}
