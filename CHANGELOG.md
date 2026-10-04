# Changelog

All notable changes to this SDK are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). A Go module carries no
version in `go.mod`; each version is released as a `vX.Y.Z` tag.

## [1.0.0] - Unreleased

**BREAKING.** Requires API contract 2.0.0.

### Added
- `Faces.CreateLivenessSession(ctx, collectionID)` returns a `LivenessSession`
  (`SessionID`, ordered `Challenges` of type `blink`, `turn_left` or
  `turn_right`, `ExpiresAt`).
- `Faces.CompleteLivenessSession(ctx, collectionID, sessionID, frames, mirrored)`
  returns a `LivenessSessionResult`: the active liveness verdict, `Steps`
  (type and passed) and, on a pass, `LivenessToken` and `LivenessTokenExpiresAt`.
  It is retried only on 429, never on a network error or a 5xx such as 503,
  since the session is used up by then.
- The error code `LIVENESS_SESSION_INVALID` (422), returned as `*APIError`.

### Changed
- Pinned API contract 2.0.0 (`ContractVersion`, `CONTRACT_VERSION`,
  `contract/openapi-2.0.0.json`).

### Removed
- **BREAKING:** `ActiveLivenessResult.LivenessToken` and
  `ActiveLivenessResult.LivenessTokenExpiresAt`. The stateless active liveness
  check no longer issues a token.

### Migration
`ActiveLiveness` no longer returns a token. Create a session, show its
challenges, complete it with the frames:

```go
session, err := client.Faces.CreateLivenessSession(ctx, collID)
// show session.Challenges in order and capture frames while the person performs them
result, err := client.Faces.CompleteLivenessSession(ctx, collID, session.SessionID, frames, mirrored)
// enrol with RegisterInput{LivenessToken: result.LivenessToken} when result.IsLive
```

Keep `ActiveLiveness` only where a verdict without a token is enough.
