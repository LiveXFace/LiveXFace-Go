# Serupa Go SDK

Official Go client for the [Serupa](https://serupa.ai) face recognition API.

## Installation

```bash
go get github.com/serupa/serupa-go
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

    serupa "github.com/serupa/serupa-go"
)

func main() {
    client := serupa.New(os.Getenv("SERUPA_API_KEY"))
    ctx := context.Background()

    // Register a face
    imageBytes, _ := os.ReadFile("alice.jpg")
    face, err := client.Faces.Register(ctx, "col_01abc...", serupa.RegisterInput{
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
    result, err := client.Faces.Identify(ctx, "col_01abc...", serupa.IdentifyInput{
        Image: probeBytes,
        TopK:  3,
    })
    if err != nil {
        // Check if it is an API error
        if apiErr, ok := err.(*serupa.APIError); ok {
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
client := serupa.New(
    "srp_live_xxxx",
    serupa.WithBaseURL("https://your-instance.example.com/api/v1"),
    serupa.WithTimeout(15 * time.Second),
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
| `Faces.Compare(ctx, CompareInput)` | Compare two images without enrolling |
| `Faces.BatchRegister(ctx, collectionID, []BatchItem)` | Enroll up to 20 faces in one request |

### Collections

| Method | Description |
|--------|-------------|
| `Collections.List(ctx)` | List all accessible collections |
| `Collections.Get(ctx, collectionID)` | Retrieve a collection by ID |
| `Collections.Create(ctx, CreateCollectionInput)` | Create a new collection |
| `Collections.Update(ctx, collectionID, UpdateCollectionInput)` | Update name, description, or retention |
| `Collections.Delete(ctx, collectionID)` | Delete a collection and all its faces |

## Error Handling

All methods return a standard `error`. When the server returns an API-level error,
the value is `*serupa.APIError`:

```go
result, err := client.Faces.Identify(ctx, collID, input)
if err != nil {
    var apiErr *serupa.APIError
    if errors.As(err, &apiErr) {
        fmt.Println("code:", apiErr.Code)
        fmt.Println("status:", apiErr.StatusCode)
        fmt.Println("request_id:", apiErr.RequestID)
    }
}
```

## License

MIT
