package composer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const Version = "1.0.0"

// sensitiveFields carry end-user information and are masked before anything is logged.
// nextSteps is included because a blocked checkpoint returns integration config
// (including provider client secrets) alongside the full client integration bundle.
var sensitiveFields = map[string]bool{"ip": true, "data": true, "stepData": true, "nextSteps": true}

const redactedValue = "[REDACTED]"

// RedactSensitiveFields renders a payload as JSON with end-user fields masked, so verbose
// logs show the shape of a request or response without exposing PII
func RedactSensitiveFields(payload interface{}) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return redactedValue
	}

	var decoded interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return redactedValue
	}

	redacted, err := json.Marshal(maskSensitiveFields(decoded))
	if err != nil {
		return redactedValue
	}

	return string(redacted)
}

// maskSensitiveFields walks a decoded JSON value and replaces every sensitive field
func maskSensitiveFields(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, entry := range typed {
			if sensitiveFields[key] {
				typed[key] = redactedValue
			} else {
				typed[key] = maskSensitiveFields(entry)
			}
		}
		return typed
	case []interface{}:
		for index, entry := range typed {
			typed[index] = maskSensitiveFields(entry)
		}
		return typed
	default:
		return value
	}
}

// MakeRequest performs an HTTP request with the given parameters.
// An optional requestTimeout caps the full HTTP round trip; when omitted, DefaultRequestTimeout is used.
func MakeRequest(url string, method string, headers map[string]string, body interface{}, requestTimeout ...time.Duration) (*http.Response, error) {
	timeout := defaultRequestTimeout
	if len(requestTimeout) > 0 && requestTimeout[0] > 0 {
		timeout = requestTimeout[0]
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		if Logger.IsTraceEnabled() {
			Logger.Trace(fmt.Sprintf("Request body: %s", RedactSensitiveFields(body))).Log()
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		Logger.Error("There was an error creating a request.", err).Log()
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		Logger.Error("There was an error making a request.", err).Log()
		// Return a synthetic error response similar to Node.js version
		errorResp := ComposerCheckpointResponse{
			Success: false,
			Errors: []ComposerApiError{
				{Code: 500, Message: "Internal Server Error"},
			},
		}
		jsonResp, _ := json.Marshal(errorResp)
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(bytes.NewReader(jsonResp)),
		}, nil
	}

	return resp, nil
}

// ConstructApiUrl constructs the API URL with the version appended
func ConstructApiUrl(url string, version string) string {
	// Ensure that the url ends with a '/'
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return url + version + "/"
}

// ConstructApiHeaders constructs the API request headers
func ConstructApiHeaders(token string, verificationId string, sourceToken string, customerId string, sessionId string) map[string]string {
	headers := map[string]string{
		"Dodgeball-Secret-Key": token,
		"Composer-SDK-Version": Version,
		"Composer-SDK-Source":  "go-sdk",
		"Content-Type":         "application/json",
	}

	if !IsNullOrEmpty(verificationId) {
		headers["Dodgeball-Verification-Id"] = verificationId
	}

	if !IsNullOrEmpty(sourceToken) {
		headers["Dodgeball-Source-Token"] = sourceToken
	}

	if !IsNullOrEmpty(customerId) {
		headers["Dodgeball-Customer-Id"] = customerId
	}

	if !IsNullOrEmpty(sessionId) {
		headers["Dodgeball-Session-Id"] = sessionId
	}

	return headers
}

// Sleep pauses execution for the given duration
func Sleep(duration time.Duration) {
	time.Sleep(duration)
}

// IsNullOrEmpty checks if a value is null or empty
func IsNullOrEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		return trimmed == "" || strings.ToLower(trimmed) == "null" || strings.ToLower(trimmed) == "undefined"
	case *string:
		if v == nil {
			return true
		}
		return IsNullOrEmpty(*v)
	default:
		return false
	}
}
