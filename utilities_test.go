package composer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstructApiUrl(t *testing.T) {
	url1 := ConstructApiUrl("https://api.dodgeballhq.com/", "v1")
	url2 := ConstructApiUrl("https://api.dodgeballhq.com", "v1")

	assert.Equal(t, "https://api.dodgeballhq.com/v1/", url1)
	assert.Equal(t, "https://api.dodgeballhq.com/v1/", url2)
}

func TestConstructApiHeaders(t *testing.T) {
	t.Run("with only token", func(t *testing.T) {
		headers := ConstructApiHeaders("test-secret-key", "", "", "", "")

		assert.Equal(t, "test-secret-key", headers["Dodgeball-Secret-Key"])
		assert.Equal(t, Version, headers["Composer-SDK-Version"])
		assert.Equal(t, "go-sdk", headers["Composer-SDK-Source"])
		assert.Equal(t, "application/json", headers["Content-Type"])
		assert.NotContains(t, headers, "Dodgeball-Verification-Id")
		assert.NotContains(t, headers, "Dodgeball-Source-Token")
		assert.NotContains(t, headers, "Dodgeball-Customer-Id")
		assert.NotContains(t, headers, "Dodgeball-Session-Id")
	})

	t.Run("with token and verification", func(t *testing.T) {
		headers := ConstructApiHeaders("test-secret-key", "test-verification-id", "", "", "")

		assert.Equal(t, "test-secret-key", headers["Dodgeball-Secret-Key"])
		assert.Equal(t, "test-verification-id", headers["Dodgeball-Verification-Id"])
		assert.Equal(t, Version, headers["Composer-SDK-Version"])
		assert.Equal(t, "go-sdk", headers["Composer-SDK-Source"])
		assert.Equal(t, "application/json", headers["Content-Type"])
	})

	t.Run("with token and source", func(t *testing.T) {
		headers := ConstructApiHeaders("test-secret-key", "", "test-source-token", "", "")

		assert.Equal(t, "test-secret-key", headers["Dodgeball-Secret-Key"])
		assert.Equal(t, "test-source-token", headers["Dodgeball-Source-Token"])
		assert.Equal(t, Version, headers["Composer-SDK-Version"])
		assert.Equal(t, "go-sdk", headers["Composer-SDK-Source"])
		assert.Equal(t, "application/json", headers["Content-Type"])
	})

	t.Run("with token and customer", func(t *testing.T) {
		headers := ConstructApiHeaders("test-secret-key", "", "", "test-customer-id", "")

		assert.Equal(t, "test-secret-key", headers["Dodgeball-Secret-Key"])
		assert.Equal(t, "test-customer-id", headers["Dodgeball-Customer-Id"])
		assert.Equal(t, Version, headers["Composer-SDK-Version"])
		assert.Equal(t, "go-sdk", headers["Composer-SDK-Source"])
		assert.Equal(t, "application/json", headers["Content-Type"])
	})

	t.Run("with token and sessionId", func(t *testing.T) {
		headers := ConstructApiHeaders("test-secret-key", "", "", "", "test-session-id")

		assert.Equal(t, "test-secret-key", headers["Dodgeball-Secret-Key"])
		assert.Equal(t, "test-session-id", headers["Dodgeball-Session-Id"])
		assert.Equal(t, Version, headers["Composer-SDK-Version"])
		assert.Equal(t, "go-sdk", headers["Composer-SDK-Source"])
		assert.Equal(t, "application/json", headers["Content-Type"])
	})
}

func TestIsNullOrEmpty(t *testing.T) {
	t.Run("returns true for null, empty, and special strings", func(t *testing.T) {
		assert.True(t, IsNullOrEmpty(nil))
		assert.True(t, IsNullOrEmpty(""))
		assert.True(t, IsNullOrEmpty("   "))
		assert.True(t, IsNullOrEmpty("null"))
		assert.True(t, IsNullOrEmpty("NULL"))
		assert.True(t, IsNullOrEmpty("undefined"))
		assert.True(t, IsNullOrEmpty("UNDEFINED"))

		// Test with string pointer
		var nilStr *string
		assert.True(t, IsNullOrEmpty(nilStr))

		emptyStr := ""
		assert.True(t, IsNullOrEmpty(&emptyStr))

		nullStr := "null"
		assert.True(t, IsNullOrEmpty(&nullStr))
	})

	t.Run("returns false for non-empty strings and non-string values", func(t *testing.T) {
		assert.False(t, IsNullOrEmpty("hello"))
		assert.False(t, IsNullOrEmpty(0))
		assert.False(t, IsNullOrEmpty(false))
		assert.False(t, IsNullOrEmpty(map[string]interface{}{}))
		assert.False(t, IsNullOrEmpty([]interface{}{}))

		// Test with string pointer to valid string
		validStr := "hello"
		assert.False(t, IsNullOrEmpty(&validStr))
	})
}
