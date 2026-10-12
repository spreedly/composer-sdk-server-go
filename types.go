package composer

import (
	"fmt"
	"time"
)

// ComposerApiVersion represents the version of the Composer API
type ComposerApiVersion string

const (
	V1 ComposerApiVersion = "v1"
)

// DodgeballApiVersion is an alias for backward compatibility with prior Dodgeball naming conventions
type DodgeballApiVersion = ComposerApiVersion

// VerificationStatus represents the current state of a verification
type VerificationStatus string

const (
	// VerificationStatusPending indicates the verification is in process on the server
	VerificationStatusPending VerificationStatus = "PENDING"

	// VerificationStatusBlocked indicates the verification is waiting on some action, for example MFA
	VerificationStatusBlocked VerificationStatus = "BLOCKED"

	// VerificationStatusComplete indicates the workflow evaluated successfully
	VerificationStatusComplete VerificationStatus = "COMPLETE"

	// VerificationStatusFailed indicates a workflow execution failure
	VerificationStatusFailed VerificationStatus = "FAILED"
)

// VerificationOutcome represents the result of a verification
type VerificationOutcome string

const (
	VerificationOutcomeApproved   VerificationOutcome = "APPROVED"
	VerificationOutcomeDenied     VerificationOutcome = "DENIED"
	VerificationOutcomePending    VerificationOutcome = "PENDING"
	VerificationOutcomeError      VerificationOutcome = "ERROR"
	VerificationOutcomeUsageLimit VerificationOutcome = "USAGE_LIMIT"
	VerificationOutcomeAPILimit   VerificationOutcome = "API_LIMIT"
)

// ComposerConfig contains configuration options for initializing the Composer SDK.
//
// ApiVersion: The version of the Composer API to use. This is required and should be set to "v1" for the current API version.
// ApiUrl: Optional URL to the Composer API. This is only required for self-hosted distributions. For the standard cloud-hosted version, the SDK will use the default API URL.
// LogLevel: Optional log level for the SDK, which can be TRACE, INFO, ERROR, or NONE. The default log level is INFO.
// IsEnabled: Optional flag to enable or disable the SDK. If set to false, all SDK calls will be no-ops. The default value is true.
// RequestTimeout: Optional per-request HTTP timeout (connect through response body read). When nil or non-positive, DefaultRequestTimeout (30 seconds) is used. Separate from CheckpointResponseOptions.Timeout, which controls checkpoint polling budget.
type ComposerConfig struct {
	ApiVersion     ComposerApiVersion
	ApiUrl         string
	LogLevel       ComposerLogLevel
	IsEnabled      *bool
	RequestTimeout *time.Duration
}

// DodgeballConfig is an alias for backward compatibility with prior Dodgeball naming conventions
type DodgeballConfig = ComposerConfig

// CheckpointEvent represents the event data sent to a checkpoint, including IP and arbitrary data.
//
// Ip: The IP address associated with the event, used for geolocation and risk analysis.
// Data: A key-value map of any additional data relevant to the checkpoint event, which will be digested into the Composer Vocabulary for use in policies and rules.
type CheckpointEvent struct {
	Ip   string                 `json:"ip"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// TrackEvent represents a tracking event to be sent to Spreedly.
//
// Type: The name of the event, which may be any string under 256 characters, that indicates what took place.
// Data: Any arbitrary data to track, which will be digested into the Composer Vocabulary.
// EventTime: Optional timestamp in milliseconds since Epoch indicating when the event took place. If not provided, the current time will be used.
type TrackEvent struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	EventTime *int64                 `json:"eventTime,omitempty"`
}

// TrackOptions contains options for sending a tracking event, including session/user/source identifiers.
//
// Event: The event data to be tracked, including type and arbitrary data.
// SessionId: Optional session identifier to associate with the event, which can be used for session-based policies and analysis.
// UserId: Optional user identifier to associate with the event, which can be used for user-based policies and analysis.
// SourceToken: Optional token representing the source of the event, which can be used for source-based policies and analysis.
type TrackOptions struct {
	Event       TrackEvent
	SessionId   string
	UserId      string
	SourceToken string
}

// CheckpointResponseOptions contains options for customizing checkpoint response behavior.
//
// Sync: If true, the checkpoint call will wait for a response from the server before returning.
// Timeout: Optional timeout in milliseconds for waiting for a checkpoint response. Default is 600000 ms (10 minutes).
type CheckpointResponseOptions struct {
	Sync    *bool `json:"sync,omitempty"`
	Timeout *int  `json:"timeout,omitempty"`
}

// CheckpointOptions contains options for submitting a checkpoint.
//
// CheckpointName: The name of the checkpoint to evaluate, which should correspond to a checkpoint defined in your Composer policies.
// Event: The event data associated with the checkpoint evaluation, including IP and arbitrary data.
// SourceToken: Optional token representing the source of the checkpoint event.
// SessionId: Optional session identifier to associate with the checkpoint event.
// UserId: Optional user identifier to associate with the checkpoint event.
// UseVerificationId: Optional verification ID to use for this checkpoint evaluation.
// Options: Optional settings for customizing checkpoint response behavior.
type CheckpointOptions struct {
	CheckpointName    string
	Event             CheckpointEvent
	SourceToken       string
	SessionId         string
	UserId            string
	UseVerificationId string
	Options           *CheckpointResponseOptions
}

// ComposerVerification represents the verification result for a checkpoint.
//
// Id: The unique identifier for the verification.
// Status: The current status of the verification.
// Outcome: The outcome of the verification.
// StepData: Additional data associated with the verification steps.
// NextSteps: Steps the Client SDK runs next, such as MFA.
type ComposerVerification struct {
	Id        string                   `json:"id"`
	Status    VerificationStatus       `json:"status"`
	Outcome   VerificationOutcome      `json:"outcome"`
	StepData  map[string]interface{}   `json:"stepData,omitempty"`
	NextSteps []map[string]interface{} `json:"nextSteps,omitempty"`
}

// ComposerCheckpointResponse represents the response returned from a Composer checkpoint evaluation.
//
// Success: Indicates whether the checkpoint evaluation was successful.
// Errors: An array of any errors that occurred during the checkpoint evaluation.
// Version: The version of the Composer API that processed the checkpoint request.
// Verification: The verification result for the checkpoint.
// IsTimeout: Indicates whether the checkpoint evaluation timed out.
type ComposerCheckpointResponse struct {
	Success      bool                  `json:"success"`
	Errors       []ComposerApiError    `json:"errors"`
	Version      ComposerApiVersion    `json:"version"`
	Verification *ComposerVerification `json:"verification,omitempty"`
	IsTimeout    *bool                 `json:"isTimeout,omitempty"`
}

// ComposerApiError represents an error returned by the Composer API.
//
// Code: The error code returned by the API.
// Message: A human-readable message describing the error.
type ComposerApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error types

// ComposerMissingConfigError represents a missing configuration error
type ComposerMissingConfigError struct {
	ConfigName string
	Value      interface{}
}

func (e *ComposerMissingConfigError) Error() string {
	return fmt.Sprintf("Composer SDK Error\nMissing configuration: %s\nProvided Value: %v", e.ConfigName, e.Value)
}

// ComposerInvalidConfigError represents an invalid configuration error
type ComposerInvalidConfigError struct {
	ConfigName    string
	Value         interface{}
	AllowedValues []string
}

func (e *ComposerInvalidConfigError) Error() string {
	return fmt.Sprintf("Composer SDK Error\nInvalid configuration: %s\nProvided value: %v\nAllowed values: %v", e.ConfigName, e.Value, e.AllowedValues)
}

// ComposerMissingParameterError represents a missing parameter error
type ComposerMissingParameterError struct {
	Parameter string
}

func (e *ComposerMissingParameterError) Error() string {
	return fmt.Sprintf("Composer SDK Error\nMissing parameter: %s", e.Parameter)
}
