package composer

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Regression coverage for the safe logging default: an integrator who passes no
// config must never see request bodies, response bodies or event.Ip in their logs.
const (
	loggingTestIp    = "203.0.113.7"
	loggingTestEmail = "shopper@example.com"
)

// runCheckpointCapturingLogs runs a checkpoint against a stub API and returns everything the SDK logged.
func runCheckpointCapturingLogs(t *testing.T, config *ComposerConfig) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ComposerCheckpointResponse{
			Success: true,
			Version: V1,
			Verification: &ComposerVerification{
				Id:       "verification123",
				Status:   VerificationStatusComplete,
				Outcome:  VerificationOutcomeApproved,
				StepData: map[string]interface{}{"email": loggingTestEmail},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	composer, err := NewComposer("test-secret-key", config)
	assert.NoError(t, err)
	composer.Config.ApiUrl = server.URL + "/"

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	_, err = composer.Checkpoint(&CheckpointOptions{
		CheckpointName: "CHECKOUT",
		Event: CheckpointEvent{
			Ip:   loggingTestIp,
			Data: map[string]interface{}{"email": loggingTestEmail},
		},
		SessionId: "session123",
	})
	assert.NoError(t, err)

	return logs.String()
}

func TestDefaultLogLevelEmitsNothingForACheckpoint(t *testing.T) {
	defer func(original Severity) { Logger.FilterLevel = original }(Logger.FilterLevel)

	output := runCheckpointCapturingLogs(t, nil)

	assert.Equal(t, SeverityInfo, Logger.FilterLevel)
	assert.Empty(t, output)
}

func TestTraceLogLevelLogsRedactedPayloads(t *testing.T) {
	defer func(original Severity) { Logger.FilterLevel = original }(Logger.FilterLevel)

	output := runCheckpointCapturingLogs(t, &ComposerConfig{LogLevel: ComposerLogLevelTrace})

	assert.Contains(t, output, string(VerificationStatusComplete))
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, loggingTestIp)
	assert.NotContains(t, output, loggingTestEmail)
}

func TestRedactSensitiveFieldsMasksEndUserFields(t *testing.T) {
	redacted := RedactSensitiveFields(map[string]interface{}{
		"event": map[string]interface{}{
			"type": "CHECKOUT",
			"ip":   loggingTestIp,
			"data": map[string]interface{}{"email": loggingTestEmail},
		},
		"verification": map[string]interface{}{
			"id":       "verification123",
			"stepData": map[string]interface{}{"email": loggingTestEmail},
		},
	})

	assert.Contains(t, redacted, "CHECKOUT")
	assert.Contains(t, redacted, "verification123")
	assert.Contains(t, redacted, "[REDACTED]")
	assert.NotContains(t, redacted, loggingTestIp)
	assert.NotContains(t, redacted, loggingTestEmail)
}

func TestRedactSensitiveFieldsMasksNextSteps(t *testing.T) {
	redacted := RedactSensitiveFields(map[string]interface{}{
		"verification": map[string]interface{}{
			"id":     "verification123",
			"status": "BLOCKED",
			"nextSteps": []interface{}{
				map[string]interface{}{
					"name":    "STRIPE_IDENTITY",
					"config":  map[string]interface{}{"clientSecret": "vs_secret_value"},
					"content": map[string]interface{}{"text": `console.log("bundle");`},
				},
			},
		},
	})

	assert.Contains(t, redacted, "BLOCKED")
	assert.Contains(t, redacted, "[REDACTED]")
	assert.NotContains(t, redacted, "vs_secret_value")
}

func TestIsTraceEnabledTracksTheFilterLevel(t *testing.T) {
	defer func(original Severity) { Logger.FilterLevel = original }(Logger.FilterLevel)

	Logger.FilterLevel = SeverityInfo
	assert.False(t, Logger.IsTraceEnabled())

	Logger.FilterLevel = SeverityTrace
	assert.True(t, Logger.IsTraceEnabled())
}

func TestCheckpointNonJSONResponseReturnsStandardFailure(t *testing.T) {
	defer func(original Severity) { Logger.FilterLevel = original }(Logger.FilterLevel)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>Bad Gateway</html>"))
	}))
	defer server.Close()

	composer, err := NewComposer("test-secret-key", &ComposerConfig{ApiUrl: server.URL + "/"})
	assert.NoError(t, err)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	response, err := composer.Checkpoint(&CheckpointOptions{
		CheckpointName: "CHECKOUT",
		Event:          CheckpointEvent{Ip: loggingTestIp},
		SessionId:      "session123",
	})

	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Equal(t, 500, response.Errors[0].Code)
	assert.Contains(t, logs.String(), "not valid JSON")
	assert.NotContains(t, logs.String(), "Bad Gateway")
}

func TestCheckpointPollNonJSONResponsesCountTowardMaxFailures(t *testing.T) {
	defer func(original Severity) { Logger.FilterLevel = original }(Logger.FilterLevel)

	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ComposerCheckpointResponse{
				Success: true,
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusPending,
					Outcome: VerificationOutcomePending,
				},
			})
			return
		}
		polls++
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>Bad Gateway</html>"))
	}))
	defer server.Close()

	composer, err := NewComposer("test-secret-key", &ComposerConfig{ApiUrl: server.URL + "/", LogLevel: ComposerLogLevelNone})
	assert.NoError(t, err)

	response, err := composer.Checkpoint(&CheckpointOptions{
		CheckpointName: "CHECKOUT",
		Event:          CheckpointEvent{Ip: loggingTestIp},
		SessionId:      "session123",
	})

	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Equal(t, 503, response.Errors[0].Code)
	assert.True(t, response.IsTimeout != nil && *response.IsTimeout)
	assert.Equal(t, MaxFailureCount, polls)
}
