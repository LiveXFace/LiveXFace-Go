<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/logo-white.svg">
    <img src="docs/brand/logo.svg" alt="LiveXFace" width="220">
  </picture>
</p>

# LiveXFace Go SDK

Official Go client for the [LiveXFace](https://livexface.com) face recognition API.

## Installation

```bash
go get github.com/livexface/livexface-go
```

Requires Go 1.22 or later.

Validated against API contract 1.0.0 (`/openapi.json` `info.version`), exposed as `livexface.ContractVersion`. The test suite calls every client method against the pinned contract in `contract/` and fails if a method or path is missing from it or a required field is not sent; to move to a new contract, copy the release asset `openapi-<version>.json` into `contract/` and update `CONTRACT_VERSION` and the `ContractVersion` constant.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    livexface "github.com/livexface/livexface-go"
)

func main() {
    client := livexface.New(os.Getenv("LIVEXFACE_API_KEY"))
    ctx := context.Background()

    // Register a face
    imageBytes, _ := os.ReadFile("alice.jpg")
    face, err := client.Faces.Register(ctx, "col_01abc...", livexface.RegisterInput{
        ExternalID: "user_alice",
        Image:      imageBytes,
        Metadata:   map[string]interface{}{"name": "Alice Smith"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Registered face:", face.ID)

    // Identify a face
    probeBytes, _ := os.ReadFile("probe.jpg")
    result, err := client.Faces.Identify(ctx, "col_01abc...", livexface.IdentifyInput{
        Image: probeBytes,
        TopK:  3,
    })
    if err != nil {
        // Check if it is an API error
        if apiErr, ok := err.(*livexface.APIError); ok {
            fmt.Printf("API error %s (HTTP %d): %s\n", apiErr.Code, apiErr.StatusCode, apiErr.Message)
        }
        log.Fatal(err)
    }
    for _, match := range result.Matches {
        fmt.Printf("Match: %s  confidence=%.3f\n", match.ExternalID, match.Confidence)
    }
}
```

## Configuration

```go
client := livexface.New(
    "lxf_live_xxxx",
    livexface.WithBaseURL("https://your-instance.example.com/api/v1"),
    livexface.WithTimeout(15 * time.Second),
    livexface.WithRetries(3),                    // off by default; see Production retries
    livexface.WithMaxRetryDelay(30 * time.Second), // default 60s
)
```

## Method Reference

### Faces

| Method | Description |
|--------|-------------|
| `Faces.Register(ctx, collectionID, RegisterInput, ...CallOption)` | Enroll a face into a collection |
| `Faces.List(ctx, collectionID, ListOptions)` | Paginated list of faces; returns `([]*Face, total, error)` |
| `Faces.Get(ctx, collectionID, faceID)` | Retrieve a face by ID |
| `Faces.Delete(ctx, collectionID, faceID)` | Remove a face from a collection |
| `Faces.Verify(ctx, collectionID, VerifyInput)` | 1:1 verification against a stored face |
| `Faces.Identify(ctx, collectionID, IdentifyInput)` | 1:N search — return top-K matches |
| `Faces.Liveness(ctx, collectionID, image, filename)` | Passive liveness detection |
| `Faces.ActiveLiveness(ctx, collectionID, []LivenessFrame)` | Active liveness over 5–50 frames; returns a liveness token when passed |
| `Faces.Compare(ctx, CompareInput)` | Compare two images without enrolling |
| `Faces.BatchRegister(ctx, collectionID, []BatchItem, ...CallOption)` | Enroll up to 20 faces in one request |
| `Faces.BatchRegisterAsync(ctx, collectionID, []BatchItem, ...CallOption)` | Queue up to 100 faces; returns a job to poll with `GetBatchJob` |

### Liveness-gated enrolment

Collections that require liveness reject enrolment without a token from a
passed active liveness check (`LIVENESS_TOKEN_REQUIRED`). Tokens are
single-use, expire after 5 minutes, and are bound to the organization and
collection. A token that is expired, reused or for another collection returns
`LIVENESS_TOKEN_INVALID`; a token whose face does not match the enrolled image
returns `LIVENESS_FACE_MISMATCH`.

```go
frames := make([]livexface.LivenessFrame, 0, len(jpegFrames))
for _, f := range jpegFrames { // at least 5 frames captured while the user blinks and turns
    frames = append(frames, livexface.LivenessFrame{Image: f})
}
check, err := client.Faces.ActiveLiveness(ctx, collID, frames)
if err != nil {
    log.Fatal(err)
}
if !check.IsLive {
    log.Fatal("liveness check failed")
}

face, err := client.Faces.Register(ctx, collID, livexface.RegisterInput{
    ExternalID:    "user_42",
    Image:         jpegFrames[0],
    LivenessToken: check.LivenessToken,
})
```

`BatchItem` has the same optional `LivenessToken` field for `BatchRegister`
and `BatchRegisterAsync`.

### Collections

Collections are created and managed in the LiveXFace dashboard, not through
the API, so the client has no collection operations. Create one there and pass
its ID to the calls above.

## Error Handling

All methods return a standard `error`. When the server returns an API-level error,
the value is `*livexface.APIError`:

```go
result, err := client.Faces.Identify(ctx, collID, input)
if err != nil {
    var apiErr *livexface.APIError
    if errors.As(err, &apiErr) {
        fmt.Println("code:", apiErr.Code)
        fmt.Println("status:", apiErr.StatusCode)
        fmt.Println("requestId:", apiErr.RequestID)
        fmt.Println("details:", apiErr.Details)       // nil when the API sent none
        if apiErr.RetryAfter != nil {                    // seconds, from Retry-After on 429/503
            fmt.Println("retry after:", *apiErr.RetryAfter)
        }
    }
}
```

## Idempotent requests

`Register`, `BatchRegister` and `BatchRegisterAsync` accept
`livexface.WithIdempotencyKey(key)`, sent as the `Idempotency-Key` header.
The API then performs the call at most once per key for 24 hours: a repeat of
the same request gets the first response back, marked with the response header
`Idempotent-Replayed: true`, so a retried enrolment never creates a duplicate
face or a second batch job. `livexface.NewIdempotencyKey()` returns a random
UUID v4; generate one per logical enrolment and keep it for every retry of it.

- The same key with a different request fails with `IDEMPOTENCY_KEY_MISMATCH` (422).
- The same key while the first request is still running fails with
  `IDEMPOTENCY_KEY_IN_USE` (409).
- 429 and 5xx responses are not remembered, so retrying with the same key runs
  the request again.
- Any other response, including a 4xx, is remembered and replayed. To try again
  after fixing the request (say, a new image after `NO_FACE_DETECTED`), use a new key.

## Production retries

Retries are off by default. `WithRetries(n)` makes up to `n` attempts after the
first:

- 429 and 503 are retried after their `Retry-After` delay, or an exponential
  backoff with jitter (0.5 s × 2ⁿ) when there is none, capped by
  `WithMaxRetryDelay` (60 s by default).
- Network errors and other 5xx are retried only for GET, PATCH and DELETE calls
  and for calls with an idempotency key, never for other POSTs such as
  `Identify` or `Verify`.
- Other 4xx responses are never retried.
- The enrolment methods send the same idempotency key on every attempt, and
  generate one when you pass none.

After the last attempt the last error is returned.

```go
client := livexface.New(os.Getenv("LIVEXFACE_API_KEY"), livexface.WithRetries(3))

key := livexface.NewIdempotencyKey() // store it with the job to reuse it after a crash
face, err := client.Faces.Register(ctx, collID, livexface.RegisterInput{
    ExternalID: "user_42",
    Image:      imageBytes,
}, livexface.WithIdempotencyKey(key))
if err != nil {
    var apiErr *livexface.APIError
    if errors.As(err, &apiErr) {
        if apiErr.RetryAfter != nil {
            log.Printf("still busy, retry in %ds (request %s)", *apiErr.RetryAfter, apiErr.RequestID)
        }
        log.Fatalf("%s (HTTP %d, request %s)", apiErr.Code, apiErr.StatusCode, apiErr.RequestID)
    }
    log.Fatal(err) // network error after the last attempt
}
fmt.Println("Enrolled:", face.ID)
```

## License

MIT
