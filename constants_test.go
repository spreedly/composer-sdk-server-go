package composer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, V1, config.ApiVersion)
	assert.Equal(t, "https://api.dodgeballhq.com/", config.ApiUrl)
	assert.Equal(t, ComposerLogLevelInfo, config.LogLevel)
	assert.NotNil(t, config.IsEnabled)
	assert.True(t, *config.IsEnabled)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, DefaultRequestTimeout, 30*time.Second)
	assert.Equal(t, BaseCheckpointPollingWaitMs.Milliseconds(), int64(100))
	assert.Equal(t, MaxPollingWaitMs.Milliseconds(), int64(2000))
	assert.Equal(t, BaseCheckpointMaxTimeoutMs.Minutes(), float64(10))
	assert.Equal(t, MaxFailureCount, 3)
}
