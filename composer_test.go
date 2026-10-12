package composer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewComposer(t *testing.T) {
	t.Run("should require an API key", func(t *testing.T) {
		_, err := NewComposer("", nil)
		assert.Error(t, err)
		assert.IsType(t, &ComposerMissingConfigError{}, err)
	})

	t.Run("should only require an API key", func(t *testing.T) {
		composer, err := NewComposer("test-secret-key", nil)
		assert.NoError(t, err)
		assert.NotNil(t, composer)
		assert.Equal(t, "test-secret-key", composer.SecretKey)
	})

	t.Run("should accept a valid config object", func(t *testing.T) {
		requestTimeout := 45 * time.Second
		composer, err := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion:     V1,
			ApiUrl:         "https://api.composerhq.com/",
			LogLevel:       ComposerLogLevelTrace,
			RequestTimeout: &requestTimeout,
		})

		assert.NoError(t, err)
		assert.NotNil(t, composer)
		assert.Equal(t, V1, composer.Config.ApiVersion)
		assert.Equal(t, "https://api.composerhq.com/", composer.Config.ApiUrl)
		assert.Equal(t, ComposerLogLevelTrace, composer.Config.LogLevel)
		assert.Equal(t, requestTimeout, *composer.Config.RequestTimeout)
	})

	t.Run("should fail with an invalid apiVersion in config object", func(t *testing.T) {
		_, err := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion: "invalid",
		})

		assert.Error(t, err)
		assert.IsType(t, &ComposerInvalidConfigError{}, err)
	})

	t.Run("should fail with an invalid logLevel in the config object", func(t *testing.T) {
		_, err := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion: V1,
			LogLevel:   "invalid",
		})

		assert.Error(t, err)
		assert.IsType(t, &ComposerInvalidConfigError{}, err)
	})

	t.Run("should set logger filter level", func(t *testing.T) {
		// Save original filter level
		originalFilterLevel := Logger.FilterLevel
		defer func() { Logger.FilterLevel = originalFilterLevel }()

		composer, err := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion: V1,
			LogLevel:   ComposerLogLevelError,
		})

		assert.NoError(t, err)
		assert.NotNil(t, composer)
		assert.Equal(t, SeverityError, Logger.FilterLevel)
	})
}

func TestCreateErrorResponse(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	response := composer.CreateErrorResponse(500, "Test error")

	assert.False(t, response.Success)
	assert.Len(t, response.Errors, 1)
	assert.Equal(t, 500, response.Errors[0].Code)
	assert.Equal(t, "Test error", response.Errors[0].Message)
	assert.Equal(t, V1, response.Version)
	assert.NotNil(t, response.Verification)
	assert.Equal(t, "", response.Verification.Id)
	assert.Equal(t, VerificationStatusFailed, response.Verification.Status)
	assert.Equal(t, VerificationOutcomeError, response.Verification.Outcome)
}

func TestIsRunning(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return true for a pending verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.True(t, composer.IsRunning(response))
	})

	t.Run("should return true for a blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.True(t, composer.IsRunning(response))
	})

	t.Run("should return false for a denied blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for a denied pending verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for an approved blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for an approved verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for a denied verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for a failed verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: false,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusFailed,
				Outcome: VerificationOutcomeError,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for an undecided complete verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false for a timeout", func(t *testing.T) {
		isTimeout := true
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 503, Message: "Service Unavailable: Maximum retry count exceeded"},
			},
			Version: V1,
		}

		assert.False(t, composer.IsRunning(response))
	})

	t.Run("should return false when verification is nil", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success:      true,
			Errors:       []ComposerApiError{},
			Version:      V1,
			Verification: nil,
		}

		assert.False(t, composer.IsRunning(response))
	})
}

func TestIsAllowed(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return false for a pending verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsAllowed(response))
	})

	t.Run("should return false for a blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsAllowed(response))
	})

	t.Run("should return true for an approved verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.True(t, composer.IsAllowed(response))
	})

	t.Run("should return false for a denied verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.False(t, composer.IsAllowed(response))
	})

	t.Run("should return false for a failed verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: false,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusFailed,
				Outcome: VerificationOutcomeError,
			},
		}

		assert.False(t, composer.IsAllowed(response))
	})

	t.Run("should return false for an undecided complete verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsAllowed(response))
	})

	t.Run("should return false for a timeout", func(t *testing.T) {
		isTimeout := true
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 503, Message: "Service Unavailable: Maximum retry count exceeded"},
			},
			Version: V1,
		}

		assert.False(t, composer.IsAllowed(response))
	})
}

func TestIsDenied(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return false for a pending verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsDenied(response))
	})

	t.Run("should return false for a blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsDenied(response))
	})

	t.Run("should return false for an approved verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.False(t, composer.IsDenied(response))
	})

	t.Run("should return true for a denied verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.True(t, composer.IsDenied(response))
	})

	t.Run("should return false for a failed verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: false,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusFailed,
				Outcome: VerificationOutcomeError,
			},
		}

		assert.False(t, composer.IsDenied(response))
	})

	t.Run("should return false for an undecided complete verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsDenied(response))
	})

	t.Run("should return false for a timeout", func(t *testing.T) {
		isTimeout := true
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 503, Message: "Service Unavailable: Maximum retry count exceeded"},
			},
			Version: V1,
		}

		assert.False(t, composer.IsDenied(response))
	})
}

func TestIsUndecided(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return false for a pending verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsUndecided(response))
	})

	t.Run("should return false for a blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.IsUndecided(response))
	})

	t.Run("should return false for an approved verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.False(t, composer.IsUndecided(response))
	})

	t.Run("should return false for a denied verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.False(t, composer.IsUndecided(response))
	})

	t.Run("should return false for a failed verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: false,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusFailed,
				Outcome: VerificationOutcomeError,
			},
		}

		assert.False(t, composer.IsUndecided(response))
	})

	t.Run("should return true for an undecided complete verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.True(t, composer.IsUndecided(response))
	})

	t.Run("should return false for a timeout", func(t *testing.T) {
		isTimeout := true
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 503, Message: "Service Unavailable: Maximum retry count exceeded"},
			},
			Version: V1,
		}

		assert.False(t, composer.IsUndecided(response))
	})
}

func TestHasError(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return false for a pending verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.HasError(response))
	})

	t.Run("should return false for a blocked verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusBlocked,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.HasError(response))
	})

	t.Run("should return false for an approved verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.False(t, composer.HasError(response))
	})

	t.Run("should return false for a denied verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeDenied,
			},
		}

		assert.False(t, composer.HasError(response))
	})

	t.Run("should return true for a failed verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: false,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusFailed,
				Outcome: VerificationOutcomeError,
			},
		}

		assert.True(t, composer.HasError(response))
	})

	t.Run("should return false for an undecided complete verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.HasError(response))
	})

	t.Run("should return true for a timeout", func(t *testing.T) {
		isTimeout := true
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 503, Message: "Service Unavailable: Maximum retry count exceeded"},
			},
			Version: V1,
		}

		assert.True(t, composer.HasError(response))
	})

	t.Run("should return true for a successful response with an errored verification", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusFailed,
				Outcome: VerificationOutcomeError,
			},
		}

		assert.True(t, composer.HasError(response))
	})

	t.Run("should return true for an unsuccessful response with no verification or errors", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: false,
			Errors:  []ComposerApiError{},
			Version: V1,
		}

		assert.True(t, composer.HasError(response))
	})

	t.Run("should return false for a pending verification with errors", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors: []ComposerApiError{
				{Code: 504, Message: "Server timeout"},
			},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusPending,
				Outcome: VerificationOutcomePending,
			},
		}

		assert.False(t, composer.HasError(response))
	})
}

func TestIsTimeout(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return false for successful responses", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success: true,
			Errors:  []ComposerApiError{},
			Version: V1,
			Verification: &ComposerVerification{
				Id:      "test-verification-id",
				Status:  VerificationStatusComplete,
				Outcome: VerificationOutcomeApproved,
			},
		}

		assert.False(t, composer.IsTimeout(response))
	})

	t.Run("should return true for timeout responses", func(t *testing.T) {
		isTimeout := true
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 503, Message: "Service Unavailable: Maximum retry count exceeded"},
			},
			Version: V1,
		}

		assert.True(t, composer.IsTimeout(response))
	})

	t.Run("should return false when IsTimeout is false", func(t *testing.T) {
		isTimeout := false
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: &isTimeout,
			Errors: []ComposerApiError{
				{Code: 500, Message: "Internal Server Error"},
			},
			Version: V1,
		}

		assert.False(t, composer.IsTimeout(response))
	})

	t.Run("should return false when IsTimeout is nil", func(t *testing.T) {
		response := &ComposerCheckpointResponse{
			Success:   false,
			IsTimeout: nil,
			Errors: []ComposerApiError{
				{Code: 500, Message: "Internal Server Error"},
			},
			Version: V1,
		}

		assert.False(t, composer.IsTimeout(response))
	})
}

func TestCheckpointValidation(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should return error when checkpointName is missing", func(t *testing.T) {
		_, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.Error(t, err)
		assert.IsType(t, &ComposerMissingParameterError{}, err)
	})

	t.Run("should accept a checkpoint without event.ip, since the API treats it as optional", func(t *testing.T) {
		disabled := false
		disabledComposer, _ := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion: V1,
			IsEnabled:  &disabled,
		})

		response, err := disabledComposer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event:          CheckpointEvent{Ip: ""},
			SessionId:      "session123",
		})

		assert.NoError(t, err)
		assert.Equal(t, "COMPOSER_IS_DISABLED", response.Verification.Id)
	})

	t.Run("should return error when both sessionId and sourceToken are missing", func(t *testing.T) {
		_, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId:   "",
			SourceToken: "",
		})

		assert.Error(t, err)
		assert.IsType(t, &ComposerMissingParameterError{}, err)
	})

	t.Run("should return approved response when SDK is disabled", func(t *testing.T) {
		disabled := false
		composer, _ := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion: V1,
			IsEnabled:  &disabled,
		})

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, "COMPOSER_IS_DISABLED", response.Verification.Id)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.Equal(t, VerificationOutcomeApproved, response.Verification.Outcome)
	})
}

func TestEventValidation(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should skip when options is nil", func(t *testing.T) {
		err := composer.Event(nil)

		assert.NoError(t, err)
	})

	t.Run("should skip when event type is missing", func(t *testing.T) {
		err := composer.Event(&TrackOptions{
			Event: TrackEvent{
				Type: "",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
	})

	t.Run("should skip when session id and source token are missing", func(t *testing.T) {
		err := composer.Event(&TrackOptions{
			Event: TrackEvent{Type: "TEST_EVENT"},
		})

		assert.NoError(t, err)
	})
}
