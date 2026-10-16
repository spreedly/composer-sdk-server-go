# Spreedly Composer Server SDK for Go

## Table of Contents

- [Purpose](#purpose)
- [Prerequisites](#prerequisites)
- [Related](#related)
- [Installation](#installation)
- [Usage](#usage)
- [API](#api)
- [Development](#development)
- [License](#license)
- [Support](#support)

## Purpose

[Spreedly Composer](https://developer.spreedly.com) enables developers to decouple security logic from their application code. This has several benefits including:

- The ability to toggle and compare security services like fraud engines, MFA, KYC, and bot prevention.
- Faster responses to new attacks. When threats evolve and new vulnerabilities are identified, your application's security logic can be updated without changing a single line of code.
- The ability to put in placeholders for future security improvements while focusing on product development.
- A way to visualize all application security logic in one place.

The Composer Server SDK for Go makes integration with the Composer API easy and is maintained by the Spreedly team.

## Prerequisites

- Go 1.25 or higher
- You will need to obtain an API key for your application from the Composer developer center.

## Related

Check out the [Client SDK](https://npmjs.com/package/@dodgeball/trust-sdk-client) for how to integrate Composer into your frontend applications.

[Composer examples](https://github.com/spreedly/composer-examples) has a runnable example server for this SDK in `server/go`, and a web client that calls it.

## Installation

Use `go get` to install the Composer module:

```sh
go get github.com/spreedly/composer-sdk-server-go
```

## Usage

This example protects an order endpoint end to end. Your API calls a checkpoint before placing the order. When the workflow needs more input from the user, such as MFA, your frontend gathers it with the Client SDK and calls your API again with the verification ID. See [Identifiers](#identifiers) for what each ID is and [useVerification](#useverification) for how the second call works.

```go
// In your API
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	composer "github.com/spreedly/composer-sdk-server-go"
)

// Client IP for the risk engine. Callers must not choose this value.
// Default (TRUSTED_PROXY_COUNT=0): socket address, ignore X-Forwarded-For.
// Positive count: that many hops from the right of X-Forwarded-For; the hop must be an IP.
// Set TRUSTED_PROXY_COUNT only if your app runs behind trusted reverse proxies.
func getIP(r *http.Request) (string, error) {
	raw := os.Getenv("TRUSTED_PROXY_COUNT")
	if raw == "" {
		raw = "0"
	}
	hops, err := strconv.Atoi(raw)
	if err != nil || hops < 0 {
		return "", errors.New("Cannot derive client IP: TRUSTED_PROXY_COUNT is not a valid hop count")
	}

	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	var candidate string
	if hops == 0 {
		host, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			host = r.RemoteAddr
		}
		candidate = host
	} else if hops <= len(parts) {
		candidate = parts[len(parts)-hops]
	}

	if net.ParseIP(candidate) == nil {
		return "", errors.New("Cannot derive client IP: no valid IP for this TRUSTED_PROXY_COUNT")
	}
	return candidate, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func main() {
	// composerClient, err := composer.NewDodgeball(os.Getenv("DODGEBALL_SECRET_KEY"), nil) // The prior name is still supported.
	composerClient, err := composer.NewComposer(os.Getenv("SPREEDLY_SECRET_KEY"), nil)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("POST /api/orders", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order map[string]any `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		ip, err := getIP(r)
		if err != nil {
			log.Printf("client IP error: %v", err)
			http.Error(w, "checkpoint request failed", http.StatusInternalServerError)
			return
		}

		checkpointResponse, err := composerClient.Checkpoint(&composer.CheckpointOptions{
			CheckpointName: "PLACE_ORDER",
			Event: composer.CheckpointEvent{
				Ip:   ip,
				Data: map[string]any{"order": body.Order},
			},
			SourceToken:       r.Header.Get("X-Dodgeball-Source-Token"),
			SessionId:         r.Header.Get("X-Session-Id"),
			// UserId: user.Id, // The signed-in user's ID from your auth, once known. Never take it from a request header.
			UseVerificationId: r.Header.Get("X-Dodgeball-Verification-Id"),
		})
		if err != nil {
			log.Printf("checkpoint error: %v", err)
			http.Error(w, "checkpoint request failed", http.StatusInternalServerError)
			return
		}

		if composerClient.IsAllowed(checkpointResponse) {
			// placedOrder := database.CreateOrder(body.Order)
			writeJSON(w, http.StatusOK, map[string]any{"order": body.Order})
		} else if composerClient.IsDenied(checkpointResponse) {
			// Return the verification so the frontend can show the reason
			writeJSON(w, http.StatusForbidden, map[string]any{"verification": checkpointResponse.Verification})
		} else if composerClient.IsRunning(checkpointResponse) {
			// The workflow needs more input from the user (such as MFA or KYC)
			writeJSON(w, http.StatusAccepted, map[string]any{"verification": checkpointResponse.Verification})
		} else {
			// No verdict: decide whether to return the error, proceed, retry, or reject. See Errors.
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": checkpointResponse.Errors})
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

```js
// In your frontend, with the Client SDK
const placeOrder = async (order, previousVerificationId = null) => {
  const sourceToken = await dodgeball.getSourceToken();

  const endpointResponse = await axios.post("/api/orders", { order }, {
    headers: {
      "x-dodgeball-source-token": sourceToken,
      "x-dodgeball-verification-id": previousVerificationId
    },
    // Let the 202 and 403 responses through instead of throwing
    validateStatus: (status) => [200, 202, 403].includes(status)
  });

  if (endpointResponse.status === 200) {
    setIsOrderPlaced(true);
    return;
  }

  dodgeball.handleVerification(endpointResponse.data.verification, {
    onVerified: async (verification) => {
      // The user completed the extra step: call your API again with the verification ID
      await placeOrder(order, verification.id);
    },
    onApproved: async () => {
      setIsOrderPlaced(true);
    },
    onDenied: async (verification) => {
      setIsOrderDenied(true);
    },
    onError: async (error) => {
      setError(error);
      setIsPlacingOrder(false);
    }
  });
}
```

## API

### Configuration

---

The package requires a secret API key as the first argument to the constructor. Pass `nil` for the defaults.

```go
composerClient, err := composer.NewComposer("secret-api-key...", nil)
```

Optionally, you can pass a `ComposerConfig`. Fields you leave unset keep their defaults.

```go
requestTimeout := 60 * time.Second
composerClient, err := composer.NewComposer("secret-api-key...", &composer.ComposerConfig{
	ApiUrl:         "https://api.dodgeballhq.com",
	LogLevel:       composer.ComposerLogLevelError,
	RequestTimeout: &requestTimeout,
})
```

| Option | Default | Description |
| --- | --- | --- |
| `ApiVersion` | `V1` | The Composer API version to use. |
| `ApiUrl` | `https://api.dodgeballhq.com` | The base URL of the Spreedly Composer API. Useful for sending requests to different environments such as `https://api.sandbox.dodgeballhq.com`. |
| `LogLevel` | `ComposerLogLevelInfo` | `ComposerLogLevelTrace`, `ComposerLogLevelInfo`, `ComposerLogLevelError` or `ComposerLogLevelNone`. The SDK logs errors, plus redacted request and response payloads at `ComposerLogLevelTrace`, so `ComposerLogLevelInfo` and `ComposerLogLevelError` currently log the same thing. The level is shared by every client in the process. See [Safe logging](#safe-logging). |
| `RequestTimeout` | 30 seconds | A `*time.Duration`. How long a single HTTP request may take, from connecting through reading the response body. Applies to every call the SDK makes. A value of `0` or less is logged and replaced with the default. See [Timeouts](#timeouts). |
| `IsEnabled` | `true` | A `*bool`. When `false`, the SDK makes no API calls: `Checkpoint` returns an approved response and `Event` returns without sending anything. |

### Safe logging

---

The SDK logs through Go's standard `log` package. At the default `ComposerLogLevelInfo` level it writes no request or response payloads: a successful checkpoint logs nothing, and `ComposerLogLevelError` reports request failures. Payload logging is opt-in through `ComposerLogLevelTrace`, and even there the fields that describe your end users or carry provider credentials (`ip`, `data`, `stepData` and `nextSteps`) are replaced with `[REDACTED]`, so a verbose log shows the shape of a request or response without carrying PII. Turn `ComposerLogLevelTrace` on only in environments where you are comfortable with that extra detail.

### Identifiers

---

`Checkpoint` and `Event` both take these. Provide a `SessionId`, a `SourceToken`, or both.

| Parameter | Required | Description |
| --- | --- | --- |
| `SourceToken` | Either this or `SessionId` | A token representing the device making the request. Obtained from the [Client SDK](https://npmjs.com/package/@dodgeball/trust-sdk-client). |
| `SessionId` | Either this or `SourceToken` | The current session ID of the request. |
| `UserId` | No | The ID of the user in your database, once you know it (for example, after registration). |
| `UseVerificationId` | No | `Checkpoint` only. The ID of a verification the user already completed. See [useVerification](#useverification). |

### Call a Checkpoint

---

Checkpoints represent key moments of risk in an application and are at the core of how Composer works. A checkpoint can represent any activity deemed to be a risk. Some common examples include: login, placing an order, redeeming a coupon, posting a review, changing bank account information, making a donation, transferring funds, creating a listing.

```go
checkpointResponse, err := composerClient.Checkpoint(&composer.CheckpointOptions{
	CheckpointName: "CHECKPOINT_NAME",
	Event: composer.CheckpointEvent{
		Ip: "127.0.0.1",
		Data: map[string]any{
			"transaction":   map[string]any{"amount": 100, "currency": "USD"},
			"paymentMethod": map[string]any{"token": "ghi789"},
		},
	},
	SessionId: "session_def456",
})
```

Along with the [identifiers](#identifiers), `CheckpointOptions` takes:

| Parameter | Required | Description |
| --- | --- | --- |
| `CheckpointName` | Yes | The name of the checkpoint to call. |
| `Event` | Yes | A `CheckpointEvent`. |
| `Event.Ip` | No | The IP address of the device where the request originated. Recommended whenever an end user is present; leave it empty for checkpoints with no user, such as renewals. |
| `Event.Data` | No | Data your checkpoint's workflow uses. |
| `Options` | No | A `*CheckpointResponseOptions` with `Sync` (a `*bool`, defaults to `false`) and `Timeout` (an `*int`: how long the SDK keeps polling an unresolved verification, in milliseconds; defaults to 10 minutes). See [Timeouts](#timeouts). |

`Checkpoint` takes a pointer; don't pass `nil`.

### Timeouts

---

Two settings control timing and neither one caps the other. `RequestTimeout` limits how long a single HTTP request may take, and applies to every call the SDK makes. `Options.Timeout` limits how long the SDK keeps polling for an unresolved verification, and defaults to 10 minutes. The SDK always polls at least once.

```go
requestTimeout := 5 * time.Second // No single HTTP call may hang longer than 5 seconds
composerClient, err := composer.NewComposer("secret-api-key...", &composer.ComposerConfig{
	RequestTimeout: &requestTimeout,
})

pollingBudget := 10000 // Stop polling after 10 seconds and return whatever we have
checkpointResponse, err := composerClient.Checkpoint(&composer.CheckpointOptions{
	CheckpointName: "CHECKPOINT_NAME",
	Event:          composer.CheckpointEvent{Ip: "127.0.0.1"},
	SessionId:      "session_def456",
	Options: &composer.CheckpointResponseOptions{
		Timeout: &pollingBudget,
	},
})
```

This example leaves `Sync` unset, so it defaults to `false`, which we recommend. The first request returns right away with a pending verification, and the SDK polls until it resolves, so each request is short and `RequestTimeout` only guards against a hung connection. With `Sync` set to `true`, the first request waits on the server until the workflow finishes, so `RequestTimeout` must be longer than your slowest workflow. Otherwise a slow but healthy checkpoint comes back as a failure.

When the polling budget runs out, the verification is simply still pending, which is not an error: `IsRunning` returns `true`, and you can resume later by passing the verification ID to `UseVerificationId`. The exception is when the last poll before the budget ran out failed: you get that failure instead, with `Success` `false`, and `HasError` returns `true`.

### Checkpoint Responses

---

Calling a checkpoint creates a verification in Composer. Its `Status` and `Outcome` determine how your application should proceed. Use the [utility methods](#utility-methods) rather than reading these fields directly.

```go
type ComposerCheckpointResponse struct {
	Success      bool
	Errors       []ComposerApiError // Code, Message
	Version      ComposerApiVersion
	Verification *ComposerVerification // Id, Status, Outcome, StepData, NextSteps
	IsTimeout    *bool
}
```

#### Response fields

| Property | Description |
| --- | --- |
| `Success` | `false` when no verification result could be produced. See [Errors](#errors). A verification that failed while running still has `Success` `true`. |
| `Errors` | `ComposerApiError` values with a `Message` and a `Code` (`0` when the API doesn't send one). Usually empty when `Success` is `true`. |
| `Version` | The version of the Composer API that was used to make the request. |
| `Verification` | The verification. Can be `nil` when `Success` is `false`. |
| `Verification.Id` | The ID of the verification that was created. |
| `Verification.Status` | `VerificationStatusComplete`, `VerificationStatusPending`, `VerificationStatusBlocked` or `VerificationStatusFailed`. |
| `Verification.Outcome` | `VerificationOutcomeApproved`, `VerificationOutcomeDenied`, `VerificationOutcomePending`, `VerificationOutcomeError`, `VerificationOutcomeUsageLimit` or `VerificationOutcomeAPILimit`. |
| `Verification.StepData` | Data from the workflow's steps. |
| `Verification.NextSteps` | Steps the Client SDK runs next, such as MFA. Return the verification to your frontend unchanged so the Client SDK can start them right away. |
| `IsTimeout` | Set to `true` when polling stopped after 3 failed polls in a row. See `IsTimeout` below. |

#### Statuses and outcomes

| `Status` | `Outcome` | Utility method | Meaning |
| --- | --- | --- | --- |
| `COMPLETE` | `APPROVED` | `IsAllowed` | Let the request proceed. |
| `COMPLETE` | `DENIED` | `IsDenied` | Deny the request. Return the verification to the frontend to show the reason. |
| `BLOCKED` | `PENDING` | `IsRunning` | The workflow is waiting for input from the user, such as MFA or KYC. See [useVerification](#useverification). |
| `PENDING` | `PENDING` | `IsRunning` | Still processing when the polling budget ran out. Resume later with `UseVerificationId`. |
| `COMPLETE` | `PENDING` | `IsUndecided` | The workflow finished without reaching a verdict. Your application decides how to proceed. |
| `FAILED` | `ERROR` | `HasError` | The workflow failed while running, for example a third-party service was unavailable. |
| `FAILED` | `USAGE_LIMIT` | `HasError` | Your account reached its usage limit. |
| `FAILED` | `API_LIMIT` | `HasError` | Your account is sending too many requests right now, or is deactivated. |

### Utility Methods

---

Each method takes a checkpoint response and returns a `bool`.

| Method | Returns `true` when |
| --- | --- |
| `composerClient.IsAllowed(checkpointResponse)` | The request is allowed to proceed. |
| `composerClient.IsDenied(checkpointResponse)` | The request is denied. |
| `composerClient.IsRunning(checkpointResponse)` | No verdict yet: the workflow is still processing or waiting on the user. |
| `composerClient.IsUndecided(checkpointResponse)` | The workflow finished without a verdict. |
| `composerClient.HasError(checkpointResponse)` | `Success` is `false`, or the workflow failed while running. |
| `composerClient.IsTimeout(checkpointResponse)` | Polling stopped after 3 failed polls in a row. Running out of the polling `Timeout` is not a timeout in this sense; see [Timeouts](#timeouts). |

### Errors

---

Invalid input returns an error. Everything else comes back as a response. The one exception is when the request can't be built or the response body can't be read: `Checkpoint` then returns the error along with a `500` failure response.

| What happened | Returns an error? | What you get | Utility method |
| --- | --- | --- | --- |
| Empty secret key, or an invalid `ApiVersion` or `LogLevel` | Yes, from `NewComposer`: `*ComposerMissingConfigError` or `*ComposerInvalidConfigError` | | |
| Empty `CheckpointName`, or neither `SessionId` nor `SourceToken` | Yes, from `Checkpoint`: `*ComposerMissingParameterError`, with a `nil` response | | |
| The API rejected the request, for example there is no published workflow for the checkpoint name, or a usage or API limit was reached | No | `Success` `false`, the API's message in `Errors` | `HasError` |
| Network error, request timeout, or a response that isn't valid JSON | No | `Success` `false`, a `500` in `Errors` | `HasError` |
| 3 polls failed in a row | No | `Success` `false`, a `503` in `Errors` | `HasError`, `IsTimeout` |
| The workflow failed while running | No | `Success` `true`, status `FAILED`, outcome `ERROR` | `HasError` |
| Invalid input to `Event` | Never | Nothing; the SDK logs it (see [Track an Event](#track-an-event)) | |

For `CheckpointName`, `SessionId` and `SourceToken`, blank, whitespace-only, `"null"` and `"undefined"` strings count as empty. Use `errors.As` to check for a specific error type. When `Success` is `false` there is no verdict at all. Don't treat it as a denial; decide whether to retry, proceed, or reject.

### useVerification

---

Sometimes the workflow needs more input from the user before it can reach a verdict. For example, if the user must complete MFA, the checkpoint response has `Status` `BLOCKED` and `Outcome` `PENDING`, and `IsRunning` returns `true`. Return the verification to your frontend and pass it to the Client SDK's `handleVerification()` method, which gathers the input from the user. When the user is done, the frontend calls your API again with the verification ID, and your API passes it to `UseVerificationId`. The checkpoint then uses that verification instead of creating a new one, so the user isn't asked twice. The [Usage](#usage) example shows the full flow.

**Important Note:** To prevent replay attacks, each verification ID can only be passed to `UseVerificationId` once.

### Track an Event

---

You can track additional information about a user's journey by submitting tracking events from your server. This information will be added to the user's profile and is made available to checkpoints.

```go
err := composerClient.Event(&composer.TrackOptions{
	Event: composer.TrackEvent{
		Type: "ADD_TO_CART",
		Data: map[string]any{
			"product": map[string]any{"id": "sku_123", "quantity": 1},
		},
	},
	SessionId: "session_def456",
})
```

Along with the [identifiers](#identifiers), `TrackOptions` takes:

| Parameter | Required | Description |
| --- | --- | --- |
| `Event` | Yes | A `TrackEvent`. |
| `Event.Type` | Yes | A name for where the user is in their journey. Can be any non-blank string you choose. |
| `Event.Data` | No | Data to track. |
| `Event.EventTime` | No | An `*int64`: when the event happened, in milliseconds since the epoch. Defaults to now. |

`Event` doesn't return an error for invalid input or failed requests, so tracking can't break your request. It returns an error only if the request can't be built. Problems are logged at `ComposerLogLevelError` instead:

- If the options are `nil`, `Event.Type` is blank, or neither `SessionId` nor `SourceToken` is provided, the event is not sent and the SDK logs `Composer event was not sent: <reason>`.
- If the API rejects the event, the SDK logs `Composer event was not accepted by the API (HTTP <status>).`
- If the request fails (network error or timeout), the SDK logs `There was an error making a request.`

## Development

### Versioning

The SDK version is declared in `utilities.go` (`Version`, sent in the `Composer-SDK-Version` header). Go modules take the release version from the git tag (for example `v1.0.0`), so tag the release with the same version.

### Testing

The project uses Go's standard testing framework. Tests are located alongside the source files with a `_test.go` suffix.

To run tests:

```sh
go test -v ./...
```

To run tests with coverage:

```sh
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

To check coverage percentage:

```sh
go test -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### Building

To build the module:

```sh
go build ./...
```

To install dependencies:

```sh
go mod tidy
```

### Testing with the examples

To try local SDK changes end to end, clone [Composer examples](https://github.com/spreedly/composer-examples) next to this repository, point the `server/go` example at your checkout in its `go.mod`, and run it as the examples README describes:

```text
replace github.com/spreedly/composer-sdk-server-go => ../../../composer-sdk-server-go
```

Restart the example after each change.

## License

Apache License 2.0 -- see [LICENSE](LICENSE)

## Support

For support, please contact:

- Email: support@spreedly.com
- Documentation: https://developer.spreedly.com
