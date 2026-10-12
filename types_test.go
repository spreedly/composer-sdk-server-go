package composer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComposerApiVersion(t *testing.T) {
	assert.Equal(t, ComposerApiVersion("v1"), V1)
}

func TestVerificationStatus(t *testing.T) {
	assert.Equal(t, VerificationStatus("PENDING"), VerificationStatusPending)
	assert.Equal(t, VerificationStatus("BLOCKED"), VerificationStatusBlocked)
	assert.Equal(t, VerificationStatus("COMPLETE"), VerificationStatusComplete)
	assert.Equal(t, VerificationStatus("FAILED"), VerificationStatusFailed)
}

func TestVerificationOutcome(t *testing.T) {
	assert.Equal(t, VerificationOutcome("APPROVED"), VerificationOutcomeApproved)
	assert.Equal(t, VerificationOutcome("DENIED"), VerificationOutcomeDenied)
	assert.Equal(t, VerificationOutcome("PENDING"), VerificationOutcomePending)
	assert.Equal(t, VerificationOutcome("ERROR"), VerificationOutcomeError)
	assert.Equal(t, VerificationOutcome("USAGE_LIMIT"), VerificationOutcomeUsageLimit)
	assert.Equal(t, VerificationOutcome("API_LIMIT"), VerificationOutcomeAPILimit)
}

func TestComposerMissingConfigError(t *testing.T) {
	err := &ComposerMissingConfigError{
		ConfigName: "testConfig",
		Value:      "testValue",
	}

	assert.Contains(t, err.Error(), "Missing configuration: testConfig")
	assert.Contains(t, err.Error(), "testValue")
}

func TestComposerInvalidConfigError(t *testing.T) {
	err := &ComposerInvalidConfigError{
		ConfigName:    "testConfig",
		Value:         "invalidValue",
		AllowedValues: []string{"val1", "val2"},
	}

	assert.Contains(t, err.Error(), "Invalid configuration: testConfig")
	assert.Contains(t, err.Error(), "invalidValue")
	assert.Contains(t, err.Error(), "val1")
	assert.Contains(t, err.Error(), "val2")
}

func TestComposerMissingParameterError(t *testing.T) {
	err := &ComposerMissingParameterError{
		Parameter: "testParam",
	}

	assert.Contains(t, err.Error(), "Missing parameter: testParam")
	assert.NotContains(t, err.Error(), "Provided value")
}

func TestComposerConfig(t *testing.T) {
	enabled := true
	config := &ComposerConfig{
		ApiVersion: V1,
		ApiUrl:     "https://test.com/",
		LogLevel:   ComposerLogLevelInfo,
		IsEnabled:  &enabled,
	}

	assert.Equal(t, V1, config.ApiVersion)
	assert.Equal(t, "https://test.com/", config.ApiUrl)
	assert.Equal(t, ComposerLogLevelInfo, config.LogLevel)
	assert.True(t, *config.IsEnabled)
}

func TestCheckpointEvent(t *testing.T) {
	event := CheckpointEvent{
		Ip: "127.0.0.1",
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	assert.Equal(t, "127.0.0.1", event.Ip)
	assert.Equal(t, "value", event.Data["key"])
}

func TestTrackEvent(t *testing.T) {
	var eventTime int64 = 123456789
	event := TrackEvent{
		Type: "TEST_EVENT",
		Data: map[string]interface{}{
			"key": "value",
		},
		EventTime: &eventTime,
	}

	assert.Equal(t, "TEST_EVENT", event.Type)
	assert.Equal(t, "value", event.Data["key"])
	assert.Equal(t, int64(123456789), *event.EventTime)
}

func TestTrackOptions(t *testing.T) {
	options := TrackOptions{
		Event: TrackEvent{
			Type: "TEST_EVENT",
		},
		SessionId:   "session123",
		UserId:      "user123",
		SourceToken: "token123",
	}

	assert.Equal(t, "TEST_EVENT", options.Event.Type)
	assert.Equal(t, "session123", options.SessionId)
	assert.Equal(t, "user123", options.UserId)
	assert.Equal(t, "token123", options.SourceToken)
}

func TestCheckpointResponseOptions(t *testing.T) {
	sync := true
	timeout := 1000
	options := CheckpointResponseOptions{
		Sync:    &sync,
		Timeout: &timeout,
	}

	assert.True(t, *options.Sync)
	assert.Equal(t, 1000, *options.Timeout)
}

func TestCheckpointOptions(t *testing.T) {
	options := CheckpointOptions{
		CheckpointName: "TEST_CHECKPOINT",
		Event: CheckpointEvent{
			Ip: "127.0.0.1",
		},
		SourceToken:       "token123",
		SessionId:         "session123",
		UserId:            "user123",
		UseVerificationId: "verification123",
	}

	assert.Equal(t, "TEST_CHECKPOINT", options.CheckpointName)
	assert.Equal(t, "127.0.0.1", options.Event.Ip)
	assert.Equal(t, "token123", options.SourceToken)
	assert.Equal(t, "session123", options.SessionId)
	assert.Equal(t, "user123", options.UserId)
	assert.Equal(t, "verification123", options.UseVerificationId)
}

func TestComposerVerification(t *testing.T) {
	verification := ComposerVerification{
		Id:      "verification123",
		Status:  VerificationStatusComplete,
		Outcome: VerificationOutcomeApproved,
		StepData: map[string]interface{}{
			"step": "data",
		},
	}

	assert.Equal(t, "verification123", verification.Id)
	assert.Equal(t, VerificationStatusComplete, verification.Status)
	assert.Equal(t, VerificationOutcomeApproved, verification.Outcome)
	assert.Equal(t, "data", verification.StepData["step"])
}

func TestComposerCheckpointResponse(t *testing.T) {
	isTimeout := true
	response := ComposerCheckpointResponse{
		Success: true,
		Errors: []ComposerApiError{
			{Code: 200, Message: "Success"},
		},
		Version: V1,
		Verification: &ComposerVerification{
			Id:      "verification123",
			Status:  VerificationStatusComplete,
			Outcome: VerificationOutcomeApproved,
		},
		IsTimeout: &isTimeout,
	}

	assert.True(t, response.Success)
	assert.Len(t, response.Errors, 1)
	assert.Equal(t, 200, response.Errors[0].Code)
	assert.Equal(t, "Success", response.Errors[0].Message)
	assert.Equal(t, V1, response.Version)
	assert.NotNil(t, response.Verification)
	assert.Equal(t, "verification123", response.Verification.Id)
	assert.True(t, *response.IsTimeout)
}

func TestComposerApiError(t *testing.T) {
	err := ComposerApiError{
		Code:    404,
		Message: "Not Found",
	}

	assert.Equal(t, 404, err.Code)
	assert.Equal(t, "Not Found", err.Message)
}
