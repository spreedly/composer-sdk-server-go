package composer

import "time"

// DefaultConfig returns the default configuration for the Composer SDK
func DefaultConfig() *ComposerConfig {
	enabled := true
	return &ComposerConfig{
		ApiVersion: V1,
		ApiUrl:     "https://api.dodgeballhq.com/",
		LogLevel:   ComposerLogLevelInfo,
		IsEnabled:  &enabled,
	}
}

const (
	// DefaultRequestTimeout is the default HTTP timeout for each outbound API request.
	DefaultRequestTimeout = 30 * time.Second

	// BaseCheckpointPollingWaitMs is the default starting wait time between polling attempts before the backoff is applied
	BaseCheckpointPollingWaitMs = 100 * time.Millisecond

	// MaxPollingWaitMs is the maximum amount of time between polling attempts
	MaxPollingWaitMs = 2000 * time.Millisecond

	// BaseCheckpointMaxTimeoutMs is the default maximum timeout for checkpoint operations (10 minutes)
	BaseCheckpointMaxTimeoutMs = 10 * time.Minute

	// MaxFailureCount is the maximum number of consecutive failures before timing out
	MaxFailureCount = 3
)

// Consulted when a caller supplies no timeout. A var so tests can shorten it.
var defaultRequestTimeout = DefaultRequestTimeout
