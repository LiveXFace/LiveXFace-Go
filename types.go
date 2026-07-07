package frapiaas

import "time"

// Face represents a registered face in a collection.
type Face struct {
	ID           string                 `json:"id"`
	CollectionID string                 `json:"collection_id"`
	ExternalID   string                 `json:"external_id"`
	Metadata     map[string]interface{} `json:"metadata"`
	ImageURL     string                 `json:"image_url"`
	CreatedAt    time.Time              `json:"created_at"`
}

// FaceCollection represents a named bucket of enrolled faces.
type FaceCollection struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	FaceCount      int       `json:"face_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VerifyResult is returned by a 1:1 face verification request.
type VerifyResult struct {
	Match         bool    `json:"match"`
	Confidence    float64 `json:"confidence"`
	ThresholdUsed float64 `json:"threshold_used"`
	FaceID        string  `json:"face_id"`
}

// IdentifyMatch is a single match returned by a 1:N identification request.
type IdentifyMatch struct {
	FaceID     string                 `json:"face_id"`
	ExternalID string                 `json:"external_id"`
	Confidence float64                `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// IdentifyResult is returned by a 1:N identification request.
type IdentifyResult struct {
	Matches     []IdentifyMatch `json:"matches"`
	QueryTimeMs int             `json:"query_time_ms"`
}

// LivenessResult is returned by a passive liveness detection request.
type LivenessResult struct {
	IsLive        bool    `json:"is_live"`
	LivenessScore float64 `json:"liveness_score"`
	FaceDetected  bool    `json:"face_detected"`
	FaceCount     int     `json:"face_count"`
}

// CompareResult is returned by a face comparison request.
type CompareResult struct {
	Match      bool    `json:"match"`
	Confidence float64 `json:"confidence"`
	Threshold  float64 `json:"threshold"`
}

// BatchFaceResult holds the outcome of a single face in a batch register request.
type BatchFaceResult struct {
	ExternalID string `json:"external_id"`
	Face       *Face  `json:"face,omitempty"`
	Error      string `json:"error,omitempty"`
}

// BatchResponse is returned by batch register requests.
type BatchResponse struct {
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Results   []BatchFaceResult `json:"results"`
}

// ─── Input types ─────────────────────────────────────────────────────────────

// RegisterInput holds the parameters for registering a face.
type RegisterInput struct {
	ExternalID string
	Image      []byte
	// Filename is optional; defaults to "image.jpg".
	Filename string
	Metadata map[string]interface{}
}

// VerifyInput holds the parameters for a 1:1 verification request.
type VerifyInput struct {
	Image    []byte
	Filename string
	// FaceID is optional; when set, the query image is compared against that face.
	FaceID string
	// Threshold of 0 uses the server-side default.
	Threshold float64
}

// IdentifyInput holds the parameters for a 1:N identification request.
type IdentifyInput struct {
	Image     []byte
	Filename  string
	TopK      int
	Threshold float64
}

// CompareInput holds the parameters for a pairwise image comparison request.
type CompareInput struct {
	Image1    []byte
	Filename1 string
	Image2    []byte
	Filename2 string
	Threshold float64
}

// ListOptions provides pagination parameters for list requests.
type ListOptions struct {
	Limit  int
	Offset int
}

// CreateCollectionInput holds the parameters for creating a new collection.
type CreateCollectionInput struct {
	Name          string
	Description   string
	RetentionDays int // 0 means no retention policy
}

// UpdateCollectionInput holds the parameters for updating a collection.
// A nil RetentionDays pointer means "do not change".
type UpdateCollectionInput struct {
	Name          string
	Description   string
	RetentionDays *int
}

// BatchItem is a single entry in a batch register request.
type BatchItem struct {
	ExternalID string
	Image      []byte
	Filename   string
	Metadata   map[string]interface{}
}
