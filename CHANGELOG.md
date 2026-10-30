# Changelog

## 1.0.0 (2026-10-30)

First release as `github.com/spreedly/composer-sdk-server-go`. If you're upgrading from `github.com/dodgeballhq/dodgeball-trust-sdk-go`, read **Changed** and **Removed**.

### Added
- `ComposerConfig.RequestTimeout` (a `*time.Duration`): a timeout for each HTTP request, default 30 seconds. A value of `0` or less is logged and replaced with the default.
- `ComposerConfig.LogLevel` (`ComposerLogLevelTrace`, `Info`, `Error`, `None`; default `Info`). The SDK logs through the standard `log` package, and the level is shared by every client in the process. At `Trace`, `ip`, `data`, `stepData` and `nextSteps` are redacted.
- `VerificationOutcomeUsageLimit` and `VerificationOutcomeAPILimit`. Both come back with `Success` `false`, so `HasError` is `true`.
- `ComposerVerification.StepData` and `ComposerVerification.NextSteps`.
- `Event` logs an error when an event isn't sent or the API doesn't accept it.

### Changed
- The module is now `github.com/spreedly/composer-sdk-server-go` and the package is `composer`. `Dodgeball`, `NewDodgeball`, `DodgeballConfig`, `DodgeballApiVersion` and `DodgeballLogLevel` still work.
- Requires Go 1.25 or later.
- Renamed types: `CheckpointRequest` is `CheckpointOptions`, `CheckpointResponse` is `ComposerCheckpointResponse`, `CheckpointResponseVerification` is `ComposerVerification`, `CheckpointResponseError` is `ComposerApiError`, and `Config` is `ComposerConfig`.
- Renamed fields: `IP` is `Ip`, `SessionID` is `SessionId`, `UserID` is `UserId`, `UseVerificationID` is `UseVerificationId`, `APIVersion` is `ApiVersion`, and `APIURL` is `ApiUrl`.
- `dodgeball.New(secret, cfg)` is `composer.NewComposer(secret, cfg)` and returns `(*Composer, error)`. It returns `*ComposerMissingConfigError` for an empty secret and `*ComposerInvalidConfigError` for an unsupported `ApiVersion` or `LogLevel`. `NewConfig()` is `DefaultConfig()`, and a `nil` config uses the defaults.
- Changed field types: `TrackEvent.EventTime` is `*int64`; `TrackEvent.Data` and `CheckpointEvent.Data` are `map[string]any`; `CheckpointOptions.Options` is `*CheckpointResponseOptions`, with `Sync` a `*bool` and `Timeout` an `*int`; `ComposerConfig.IsEnabled` is a `*bool`.
- `Event` and `Checkpoint` take pointers. Don't pass `nil` to `Checkpoint`.
- `Verification` on the response is a `*ComposerVerification` and can be `nil`. `Status` and `Outcome` are the typed `VerificationStatus` and `VerificationOutcome`.
- The helpers moved from the response to the client: `resp.IsAllowed()` is now `client.IsAllowed(resp)`, and likewise for `IsRunning`, `IsDenied`, `IsUndecided`, `HasError` and `IsTimeout`.
- A `nil` `IsEnabled` means enabled. A zero-value `Config{}` used to disable the SDK.
- `Checkpoint` returns `*ComposerMissingParameterError` for an empty `CheckpointName` or when both `SessionId` and `SourceToken` are empty. Use `errors.As`. Blank strings, `"null"` and `"undefined"` count as empty. An empty `Event.Ip` is allowed.
- A failed checkpoint request returns a response with `Success` `false` and a `500` in `Errors`, and a `nil` error. Check `HasError`. If the request can't be built or the response body can't be read, `Checkpoint` also returns the error.
- Each request is made once, with no retries. Polling gives up after 3 failed polls in a row and returns a `503` with `IsTimeout` set.
- `Options.Timeout` is now only how long the SDK polls, and defaults to 10 minutes. It's no longer sent to the API, and leaving it out no longer means polling with no time limit. Polling backs off from 100 ms to a 2 second ceiling.
- `Options.Sync` is sent as given, and defaults to `false`.
- `Event` doesn't return an error for invalid input, network failures or API rejections. It logs the problem and returns `nil`, and sends nothing when the SDK is disabled. It still returns an error if the request can't be built, for example when `Data` can't be encoded as JSON.
- `IsRunning` requires the outcome to be `PENDING` as well as the status to be `PENDING` or `BLOCKED`.
- `HasError` is `true` when `Success` is `false` or the outcome is `ERROR`.
- When disabled, the placeholder verification ID is `COMPOSER_IS_DISABLED` (was `DODGEBALL_DISABLED`).

### Removed
- `Options.Webhook`. Remove it from your checkpoint options. `Checkpoint` polls a pending verification as before.
- `CheckpointEvent.Type`. The SDK sends the checkpoint name as the event type.
- `TrackResponse` and `TrackResponseError`.
- `ErrMissingCheckpointName`, `ErrMissingEventIP` and `ErrMissingSessionIDOrSourceToken`, and the constants `APIVersion`, `APIURL`, `BaseCheckpointTimeout`, `MaxPollingSleep` and `MaxRetryCount`. Use `V1` for `APIVersion`, `DefaultConfig().ApiUrl` for `APIURL`, `MaxPollingWaitMs` for `MaxPollingSleep` and `MaxFailureCount` for `MaxRetryCount`. `BaseCheckpointTimeout` has no replacement; see `Options.Timeout` above.
