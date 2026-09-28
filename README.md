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
)
```

## Method Reference

### Faces

| Method | Description |
|--------|-------------|
| `Faces.Register(ctx, collectionID, RegisterInput)` | Enroll a face into a collection |
| `Faces.List(ctx, collectionID, ListOptions)` | Paginated list of faces; returns `([]*Face, total, error)` |
| `Faces.Get(ctx, collectionID, faceID)` | Retrieve a face by ID |
| `Faces.Delete(ctx, collectionID, faceID)` | Remove a face from a collection |
| `Faces.Verify(ctx, collectionID, VerifyInput)` | 1:1 verification against a stored face |
| `Faces.Identify(ctx, collectionID, IdentifyInput)` | 1:N search — return top-K matches |
| `Faces.Liveness(ctx, collectionID, image, filename)` | Passive liveness detection |
| `Faces.ActiveLiveness(ctx, collectionID, []LivenessFrame)` | Active liveness over 5–50 frames; returns a liveness token when passed |
| `Faces.Compare(ctx, CompareInput)` | Compare two images without enrolling |
| `Faces.BatchRegister(ctx, collectionID, []BatchItem)` | Enroll up to 20 faces in one request |

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
    }
}
```

## License

MIT
