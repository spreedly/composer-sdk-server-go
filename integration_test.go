package composer

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventWithHTTPCall(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should successfully send event", func(t *testing.T) {
		// Create a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/track")
			assert.Equal(t, "test-secret-key", r.Header.Get("Dodgeball-Secret-Key"))
			assert.Equal(t, "go-sdk", r.Header.Get("Composer-SDK-Source"))

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
			})
		}))
		defer server.Close()

		// Update composer config to use test server
		composer.Config.ApiUrl = server.URL + "/"

		err := composer.Event(&TrackOptions{
			Event: TrackEvent{
				Type: "TEST_EVENT",
				Data: map[string]interface{}{
					"key": "value",
				},
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
	})

	t.Run("should not send event when SDK is disabled", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		disabled := false
		disabledComposer, _ := NewComposer("test-secret-key", &ComposerConfig{
			ApiUrl:    server.URL + "/",
			IsEnabled: &disabled,
		})

		err := disabledComposer.Event(&TrackOptions{
			Event:     TrackEvent{Type: "TEST_EVENT"},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, requests)
	})

	t.Run("should still require an event when SDK is disabled", func(t *testing.T) {
		disabled := false
		disabledComposer, _ := NewComposer("test-secret-key", &ComposerConfig{IsEnabled: &disabled})

		err := disabledComposer.Event(&TrackOptions{SessionId: "session123"})

		assert.NoError(t, err)
	})

	t.Run("should skip a nil options pointer before sending", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		err := composer.Event(nil)

		assert.NoError(t, err)
		assert.Equal(t, 0, requests)
	})

	t.Run("should skip an event without a type before sending", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		err := composer.Event(&TrackOptions{
			Event:     TrackEvent{Data: map[string]interface{}{"key": "value"}},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, requests)
	})

	t.Run("should skip an event without a session id or source token", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		err := composer.Event(&TrackOptions{
			Event: TrackEvent{Type: "TEST_EVENT"},
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, requests)
	})

	t.Run("should add timestamp if not provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body TrackEvent
			json.NewDecoder(r.Body).Decode(&body)
			assert.NotNil(t, body.EventTime)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		err := composer.Event(&TrackOptions{
			Event: TrackEvent{
				Type: "TEST_EVENT",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
	})

	t.Run("should log, not return, an HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		var logs bytes.Buffer
		log.SetOutput(&logs)
		defer log.SetOutput(os.Stderr)

		err := composer.Event(&TrackOptions{
			Event: TrackEvent{
				Type: "TEST_EVENT",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.Contains(t, logs.String(), "Composer event was not accepted by the API (HTTP 400).")
	})

	t.Run("should not report a network failure as an API rejection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		var logs bytes.Buffer
		log.SetOutput(&logs)
		defer log.SetOutput(os.Stderr)

		err := composer.Event(&TrackOptions{
			Event:     TrackEvent{Type: "TEST_EVENT"},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, strings.Count(logs.String(), "There was an error making a request."))
		assert.NotContains(t, logs.String(), "not accepted by the API")
	})

	t.Run("should not log when the API accepts the event", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		var logs bytes.Buffer
		log.SetOutput(&logs)
		defer log.SetOutput(os.Stderr)

		err := composer.Event(&TrackOptions{
			Event:     TrackEvent{Type: "TEST_EVENT"},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.Empty(t, logs.String())
	})

	t.Run("should close the response so connections are reused", func(t *testing.T) {
		var newConns, received int32
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&received, 1)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"success":true,"errors":[]}`))
		}))
		server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
			if state == http.StateNew {
				atomic.AddInt32(&newConns, 1)
			}
		}
		server.Start()
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		const events = 5
		for i := 0; i < events; i++ {
			err := composer.Event(&TrackOptions{
				Event:     TrackEvent{Type: "TEST_EVENT"},
				SessionId: "session123",
			})
			assert.NoError(t, err)
		}

		// MakeRequest turns transport failures into a nil error, so check every event arrived
		assert.Equal(t, int32(events), atomic.LoadInt32(&received))
		// An unclosed body pins its connection, so every event would open a new one
		assert.Less(t, atomic.LoadInt32(&newConns), int32(events))
	})
}

func TestCheckpointWithHTTPCall(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should successfully complete checkpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/checkpoint")

			response := ComposerCheckpointResponse{
				Success: true,
				Errors:  []ComposerApiError{},
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusComplete,
					Outcome: VerificationOutcomeApproved,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, "verification123", response.Verification.Id)
		assert.True(t, composer.IsAllowed(response))
	})

	t.Run("should leave event.ip out of the request when it's empty", func(t *testing.T) {
		var event map[string]interface{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			event = body["event"].(map[string]interface{})

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ComposerCheckpointResponse{
				Success: true,
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusComplete,
					Outcome: VerificationOutcomeApproved,
				},
			})
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		_, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			SessionId:      "session123",
		})

		assert.NoError(t, err)
		assert.NotContains(t, event, "ip")
		assert.Equal(t, "TEST_CHECKPOINT", event["type"])
	})

	t.Run("should poll for pending verification", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var response ComposerCheckpointResponse

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// First call returns pending
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			} else if strings.Contains(r.URL.Path, "/verification") {
				// Polling call returns complete
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusComplete,
						Outcome: VerificationOutcomeApproved,
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.True(t, callCount >= 2) // At least checkpoint + 1 poll
	})

	t.Run("should timeout after max failures", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// First call returns pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Polling calls fail
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		timeout := 500 // Short timeout
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &timeout,
			},
		})

		assert.NoError(t, err)
		assert.False(t, response.Success)
		assert.True(t, composer.IsTimeout(response))
	})

	t.Run("should handle blocked status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ComposerCheckpointResponse{
				Success: true,
				Errors:  []ComposerApiError{},
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusBlocked,
					Outcome: VerificationOutcomePending,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, composer.IsRunning(response))
	})

	t.Run("should return nextSteps for a blocked verification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success":true,"version":"v1","verification":{"id":"verification123","status":"BLOCKED","outcome":"PENDING","nextSteps":[{"id":"step-1","name":"MFA","url":"https://example.com/mfa.js","config":{}}]}}`))
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			SessionId:      "session123",
		})

		assert.NoError(t, err)
		assert.Equal(t, "step-1", response.Verification.NextSteps[0]["id"])

		encoded, _ := json.Marshal(response.Verification)
		assert.Contains(t, string(encoded), `"nextSteps":[{`)
	})

	t.Run("should handle nil response from server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("null")) // JSON null will unmarshal to nil response
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.False(t, response.Success)
	})

	t.Run("should reset failure count on successful poll", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var response ComposerCheckpointResponse

			if strings.Contains(r.URL.Path, "/checkpoint") {
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			} else if strings.Contains(r.URL.Path, "/verification") {
				if callCount <= 3 {
					// First 2 polls fail
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else if callCount == 4 {
					// Third poll succeeds but still pending
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusPending,
							Outcome: VerificationOutcomePending,
						},
					}
				} else {
					// Fourth poll completes
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
	})

	t.Run("should handle custom timeout value", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ComposerCheckpointResponse{
				Success: true,
				Errors:  []ComposerApiError{},
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusComplete,
					Outcome: VerificationOutcomeApproved,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		customTimeout := 5000
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &customTimeout,
			},
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
	})
}

func TestMakeRequest(t *testing.T) {
	t.Run("should successfully make GET request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		}))
		defer server.Close()

		response, err := MakeRequest(server.URL, "GET", map[string]string{}, nil)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("should successfully make POST request with body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)

			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "value", body["key"])

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		}))
		defer server.Close()

		headers := map[string]string{
			"Content-Type": "application/json",
		}
		body := map[string]interface{}{
			"key": "value",
		}

		response, err := MakeRequest(server.URL, "POST", headers, body)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("should handle invalid JSON in body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create an un-marshalable body (channel cannot be marshaled to JSON)
		body := map[string]interface{}{
			"channel": make(chan int),
		}

		_, err := MakeRequest(server.URL, "POST", map[string]string{}, body)

		assert.Error(t, err)
	})

	t.Run("should handle network errors", func(t *testing.T) {
		// Use an invalid URL to trigger network error
		response, err := MakeRequest("http://invalid-host-that-does-not-exist-12345.com", "GET", map[string]string{}, nil)

		// Should return a synthetic error response
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
	})

	t.Run("should set custom headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "custom-value", r.Header.Get("Custom-Header"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		headers := map[string]string{
			"Custom-Header": "custom-value",
			"Content-Type":  "application/json",
		}

		response, err := MakeRequest(server.URL, "GET", headers, nil)

		assert.NoError(t, err)
		assert.NotNil(t, response)
	})
}

func stallingHTTPHandler(release <-chan struct{}) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	})
}

func newStallingTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	release := make(chan struct{})
	server := httptest.NewServer(stallingHTTPHandler(release))
	t.Cleanup(func() {
		close(release)
		server.CloseClientConnections()
		server.Close()
	})

	return server
}

func TestMakeRequestHTTPTimeout(t *testing.T) {
	t.Run("returns synthetic 500 quickly when caller overrides timeout", func(t *testing.T) {
		server := newStallingTestServer(t)

		start := time.Now()
		response, err := MakeRequest(server.URL, "GET", map[string]string{}, nil, 100*time.Millisecond)
		elapsed := time.Since(start)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		assert.Less(t, elapsed, 2*time.Second)
		assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	})

	t.Run("returns synthetic 500 quickly when caller supplies no timeout", func(t *testing.T) {
		// Shortened from the real 30s default so the test does not wait that long.
		restore := defaultRequestTimeout
		defaultRequestTimeout = 200 * time.Millisecond
		t.Cleanup(func() { defaultRequestTimeout = restore })

		server := newStallingTestServer(t)

		start := time.Now()
		response, err := MakeRequest(server.URL, "GET", map[string]string{}, nil)
		elapsed := time.Since(start)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
		assert.Less(t, elapsed, 2*time.Second)
	})
}

func TestComposerRequestTimeout(t *testing.T) {
	t.Run("uses default request timeout when config omits RequestTimeout", func(t *testing.T) {
		composer, err := NewComposer("test-secret-key", nil)
		assert.NoError(t, err)
		assert.Equal(t, DefaultRequestTimeout, composer.requestTimeout())
	})

	t.Run("uses default request timeout when RequestTimeout is not positive", func(t *testing.T) {
		for _, requestTimeout := range []time.Duration{0, -1 * time.Second} {
			composer, err := NewComposer("test-secret-key", &ComposerConfig{
				ApiVersion:     V1,
				LogLevel:       ComposerLogLevelNone,
				RequestTimeout: &requestTimeout,
			})

			assert.NoError(t, err)
			assert.Nil(t, composer.Config.RequestTimeout)
			assert.Equal(t, DefaultRequestTimeout, composer.requestTimeout())
		}
	})

	t.Run("honors RequestTimeout config override on Event", func(t *testing.T) {
		requestTimeout := 100 * time.Millisecond
		composer, err := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion:     V1,
			LogLevel:       ComposerLogLevelNone,
			RequestTimeout: &requestTimeout,
		})
		assert.NoError(t, err)

		server := newStallingTestServer(t)
		composer.Config.ApiUrl = server.URL + "/"

		start := time.Now()
		err = composer.Event(&TrackOptions{
			Event: TrackEvent{
				Type: "TEST_EVENT",
			},
			SessionId: "session123",
		})
		elapsed := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, elapsed, 2*time.Second)
	})

	t.Run("checkpoint polling budget is independent of per-request RequestTimeout", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var response ComposerCheckpointResponse
			if strings.Contains(r.URL.Path, "/checkpoint") {
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			} else {
				if callCount < 3 {
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusPending,
							Outcome: VerificationOutcomePending,
						},
					}
				} else {
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		requestTimeout := 5 * time.Second
		pollingTimeoutMs := 30000
		composer, err := NewComposer("test-secret-key", &ComposerConfig{
			ApiVersion:     V1,
			ApiUrl:         server.URL + "/",
			LogLevel:       ComposerLogLevelNone,
			RequestTimeout: &requestTimeout,
		})
		assert.NoError(t, err)

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &pollingTimeoutMs,
			},
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.GreaterOrEqual(t, callCount, 3)
	})
}

func TestSleep(t *testing.T) {
	t.Run("should sleep for specified duration", func(t *testing.T) {
		start := time.Now()
		Sleep(50 * time.Millisecond)
		elapsed := time.Since(start)

		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(50))
	})

	t.Run("should not panic with zero duration", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Sleep(0)
		})
	})
}

func TestCheckpointEdgeCases(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should handle http.NewRequest error during initial checkpoint", func(t *testing.T) {
		// Set an invalid URL with bad characters to cause http.NewRequest to fail
		composer.Config.ApiUrl = "http://[::1]%23:8080/" // Invalid URL format

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.Error(t, err)
		assert.NotNil(t, response)
		assert.False(t, response.Success)
	})

	t.Run("should handle read error during polling", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Send invalid response that will fail to read
				w.Header().Set("Content-Length", "1000")
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		timeout := 500
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &timeout,
			},
		})

		assert.NoError(t, err)
		assert.False(t, response.Success)
	})

	t.Run("should handle unmarshal error during polling", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Send invalid JSON
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("invalid json"))
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		timeout := 500
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &timeout,
			},
		})

		assert.NoError(t, err)
		assert.False(t, response.Success)
	})

	t.Run("should handle unmarshal error during initial checkpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Send invalid JSON for initial checkpoint
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json for checkpoint"))
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.False(t, response.Success)
	})

	t.Run("should handle MakeRequest error during polling", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			// Return pending for initial checkpoint
			response := ComposerCheckpointResponse{
				Success: true,
				Errors:  []ComposerApiError{},
				Version: V1,
				Verification: &ComposerVerification{
					// An invalid URL escape in the ID makes http.NewRequest fail when polling
					Id:      "verification%zz",
					Status:  VerificationStatusPending,
					Outcome: VerificationOutcomePending,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		// Should succeed with pending or timeout
		assert.NotNil(t, response)
	})

	t.Run("should continue polling and reset failure count on successful response", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var response ComposerCheckpointResponse

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Initial checkpoint returns pending
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			} else if callCount == 2 {
				// First poll: return nil verification (causes failure)
				response = ComposerCheckpointResponse{
					Success:      true,
					Errors:       []ComposerApiError{},
					Version:      V1,
					Verification: nil,
				}
			} else if callCount == 3 {
				// Second poll: return pending with verification (should reset failure count and enter else branch)
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			} else {
				// Finally return complete
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusComplete,
						Outcome: VerificationOutcomeApproved,
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.True(t, callCount >= 3)
	})

	t.Run("should reset failure count when polling returns pending verification", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Initial checkpoint returns pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Polling: return successful response with pending verification
				// This should trigger the else branch that resets numFailures
				if callCount <= 3 {
					response := ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusPending,
							Outcome: VerificationOutcomePending,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				} else {
					response := ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				}
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		// Should have polled multiple times
		assert.True(t, callCount > 3)
	})

	t.Run("should execute else branch when verification is pending during poll", func(t *testing.T) {
		pollCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var response ComposerCheckpointResponse

			if strings.Contains(r.URL.Path, "/verification/") {
				pollCount++
				// Polling endpoint: always return pending for first 2 polls, then complete
				if pollCount <= 2 {
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusPending,
							Outcome: VerificationOutcomePending,
						},
					}
				} else {
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
				}
			} else {
				// Initial checkpoint: return pending
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.True(t, pollCount >= 2, "Should have polled at least twice")
	})

	t.Run("should reset failure count when polling succeeds after failures", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Initial checkpoint returns pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Polling requests
				var response ComposerCheckpointResponse
				switch callCount {
				case 2:
					// First poll: unsuccessful response (increments failure count)
					response = ComposerCheckpointResponse{
						Success: false,
						Errors: []ComposerApiError{
							{Code: 500, Message: "Server Error"},
						},
						Version: V1,
					}
				case 3:
					// Second poll: successful response with pending status
					// This should hit the else branch and reset numFailures = 0
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusPending,
							Outcome: VerificationOutcomePending,
						},
					}
				default:
					// Third poll: complete
					response = ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.Equal(t, VerificationOutcomeApproved, response.Verification.Outcome)
		// Should have made 4 calls: 1 checkpoint + 3 polls
		assert.Equal(t, 4, callCount)
	})

	t.Run("should handle unsuccessful response during polling", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Return unsuccessful response
				response := ComposerCheckpointResponse{
					Success: false,
					Errors: []ComposerApiError{
						{Code: 500, Message: "Error"},
					},
					Version: V1,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		timeout := 500
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &timeout,
			},
		})

		assert.NoError(t, err)
		assert.False(t, response.Success)
	})

	t.Run("should handle read error on initial checkpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		// Should return error because reading fails
		assert.Error(t, err)
		assert.NotNil(t, response)
		assert.False(t, response.Success)
	})

	t.Run("should handle MakeRequest error on initial checkpoint", func(t *testing.T) {
		composer.Config.ApiUrl = "http://invalid-host-12345.com/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.False(t, response.Success)
	})

	t.Run("should handle no timeout specified (uses default)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ComposerCheckpointResponse{
				Success: true,
				Errors:  []ComposerApiError{},
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusComplete,
					Outcome: VerificationOutcomeApproved,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options:   nil, // No options at all
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
	})

	t.Run("should handle zero or negative timeout (uses default)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ComposerCheckpointResponse{
				Success: true,
				Errors:  []ComposerApiError{},
				Version: V1,
				Verification: &ComposerVerification{
					Id:      "verification123",
					Status:  VerificationStatusComplete,
					Outcome: VerificationOutcomeApproved,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		zeroTimeout := 0
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &zeroTimeout,
			},
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
	})
}

func TestLoggerEdgeCases(t *testing.T) {
	t.Run("SetParameter should handle nil Parameters", func(t *testing.T) {
		entry := &LogEntry{
			Message:    "test",
			Severity:   SeverityInfo,
			Parameters: nil, // Explicitly nil
		}

		result := entry.SetParameter("key", "value")

		assert.NotNil(t, entry.Parameters)
		assert.Equal(t, "value", entry.Parameters["key"])
		assert.Equal(t, entry, result)
	})
}

func TestMakeRequestEdgeCases(t *testing.T) {
	t.Run("should handle invalid URL for request creation", func(t *testing.T) {
		// Use an invalid URL that will cause http.NewRequest to fail
		_, err := MakeRequest("://invalid", "GET", map[string]string{}, nil)

		assert.Error(t, err)
	})

	t.Run("should handle invalid method", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Use an invalid HTTP method that will cause http.NewRequest to fail
		_, err := MakeRequest(server.URL, "INVALID\nMETHOD", map[string]string{}, nil)

		assert.Error(t, err)
	})
}

func TestCheckpointPollingTimeout(t *testing.T) {
	composer, _ := NewComposer("test-secret-key", nil)

	t.Run("should timeout when hasNeverPolled prevents immediate exit", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Return pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// Delay the response to simulate timeout
				time.Sleep(200 * time.Millisecond)
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		// Very short timeout - should timeout but still poll at least once
		shortTimeout := 1
		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
			Options: &CheckpointResponseOptions{
				Timeout: &shortTimeout,
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)
		// Should have called at least twice (initial + at least one poll)
		assert.True(t, callCount >= 2)
	})

	t.Run("should hit max polling wait cap", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var response ComposerCheckpointResponse

			if callCount <= 5 {
				// Keep returning pending for several polls
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
			} else {
				// Finally complete
				response = ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusComplete,
						Outcome: VerificationOutcomeApproved,
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		// Should have polled multiple times, hitting the exponential backoff cap
		assert.True(t, callCount >= 5)
	})

	t.Run("should return unsuccessful response from initial checkpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ComposerCheckpointResponse{
				Success: false,
				Errors: []ComposerApiError{
					{Code: 400, Message: "Bad Request"},
				},
				Version: V1,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.False(t, response.Success)
		assert.Len(t, response.Errors, 1)
		assert.Equal(t, 400, response.Errors[0].Code)
	})

	t.Run("should handle verification resolved with nil verification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ComposerCheckpointResponse{
				Success:      true,
				Errors:       []ComposerApiError{},
				Version:      V1,
				Verification: nil, // Nil verification means it's resolved
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Nil(t, response.Verification)
	})

	t.Run("should handle unsuccessful response during polling", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Return pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// For polling - return unsuccessful response twice then success
				if callCount <= 3 {
					response := ComposerCheckpointResponse{
						Success: false,
						Errors: []ComposerApiError{
							{Code: 500, Message: "Temporary error"},
						},
						Version: V1,
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				} else {
					// Finally resolve
					response := ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				}
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.Equal(t, VerificationOutcomeApproved, response.Verification.Outcome)
	})

	t.Run("should handle nil verification in successful polling response", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Return pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// For polling - return successful response but with nil verification twice, then resolved
				if callCount <= 3 {
					response := ComposerCheckpointResponse{
						Success:      true,
						Errors:       []ComposerApiError{},
						Version:      V1,
						Verification: nil, // nil verification in successful response
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				} else {
					// Finally resolve properly
					response := ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				}
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.Equal(t, VerificationOutcomeApproved, response.Verification.Outcome)
	})

	t.Run("should continue polling when verification still pending", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			if strings.Contains(r.URL.Path, "/checkpoint") {
				// Return pending
				response := ComposerCheckpointResponse{
					Success: true,
					Errors:  []ComposerApiError{},
					Version: V1,
					Verification: &ComposerVerification{
						Id:      "verification123",
						Status:  VerificationStatusPending,
						Outcome: VerificationOutcomePending,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				// For polling - return pending twice, then complete
				if callCount <= 3 {
					response := ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusPending, // Still pending
							Outcome: VerificationOutcomePending,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				} else {
					// Finally complete
					response := ComposerCheckpointResponse{
						Success: true,
						Errors:  []ComposerApiError{},
						Version: V1,
						Verification: &ComposerVerification{
							Id:      "verification123",
							Status:  VerificationStatusComplete,
							Outcome: VerificationOutcomeApproved,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(response)
				}
			}
		}))
		defer server.Close()

		composer.Config.ApiUrl = server.URL + "/"

		response, err := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST_CHECKPOINT",
			Event: CheckpointEvent{
				Ip: "127.0.0.1",
			},
			SessionId: "session123",
		})

		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, VerificationStatusComplete, response.Verification.Status)
		assert.Equal(t, VerificationOutcomeApproved, response.Verification.Outcome)
	})
}

func TestCheckpointPollingWithNilVerification(t *testing.T) {
	t.Run("should handle nil verification on poll response and eventually timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "checkpoint") {
				// Initial response: PENDING, triggers polling
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"errors":  []interface{}{},
					"version": "v1",
					"verification": map[string]interface{}{
						"id":      "poll-nil-id",
						"status":  "PENDING",
						"outcome": "PENDING",
					},
				})
			} else {
				// Poll response: success=true but verification explicitly null → Verification == nil
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success":      true,
					"errors":       []interface{}{},
					"version":      "v1",
					"verification": nil,
				})
			}
		}))
		defer server.Close()

		composer, _ := NewComposer("test-secret-key", &ComposerConfig{
			ApiUrl:   server.URL + "/",
			LogLevel: ComposerLogLevelNone,
		})

		// No custom timeout: polling runs until MaxFailureCount is reached
		// (each poll returns nil verification → numFailures++)
		response, _ := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST",
			Event:          CheckpointEvent{Ip: "127.0.0.1", Data: map[string]interface{}{}},
			SessionId:      "session-123",
		})

		assert.NotNil(t, response)
		assert.False(t, response.Success)
	})

	t.Run("should count a poll that leaves out verification as a failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "checkpoint") {
				w.Write([]byte(`{"success":true,"errors":[],"version":"v1","verification":{"id":"poll-missing-id","status":"PENDING","outcome":"PENDING"}}`))
			} else {
				w.Write([]byte(`{"success":true,"errors":[],"version":"v1"}`))
			}
		}))
		defer server.Close()

		composer, _ := NewComposer("test-secret-key", &ComposerConfig{
			ApiUrl:   server.URL + "/",
			LogLevel: ComposerLogLevelNone,
		})

		// Long enough for three failed polls (~700ms), short enough that a regression returns
		// the stale PENDING response quickly instead of polling for the 10-minute default
		pollingBudgetMs := 2000
		response, _ := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST",
			Event:          CheckpointEvent{Data: map[string]interface{}{}},
			SessionId:      "session-123",
			Options: &CheckpointResponseOptions{
				Timeout: &pollingBudgetMs,
			},
		})

		assert.True(t, composer.IsTimeout(response))
	})

	t.Run("should not carry stepData over from an earlier poll", func(t *testing.T) {
		polls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "checkpoint") {
				w.Write([]byte(`{"success":true,"errors":[],"version":"v1","verification":{"id":"poll-merge-id","status":"PENDING","outcome":"PENDING","stepData":{"stale":"yes"}}}`))
				return
			}
			polls++
			w.Write([]byte(`{"success":true,"errors":[],"version":"v1","verification":{"id":"poll-merge-id","status":"COMPLETE","outcome":"APPROVED","stepData":{"fresh":"yes"}}}`))
		}))
		defer server.Close()

		composer, _ := NewComposer("test-secret-key", &ComposerConfig{
			ApiUrl:   server.URL + "/",
			LogLevel: ComposerLogLevelNone,
		})

		response, _ := composer.Checkpoint(&CheckpointOptions{
			CheckpointName: "TEST",
			Event:          CheckpointEvent{Data: map[string]interface{}{}},
			SessionId:      "session-123",
		})

		assert.Equal(t, 1, polls)
		assert.Equal(t, map[string]interface{}{"fresh": "yes"}, response.Verification.StepData)
	})
}
