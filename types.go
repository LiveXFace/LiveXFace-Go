package livexface

import "time"

// Face represents a registered face in a collection.
type Face struct {
	ID           string                 `json:"id"`
	CollectionID string                 `json:"collectionId"`
	ExternalID   string                 `json:"externalId"`
	Metadata     map[string]interface{} `json:"metadata"`
	ImageURL     string                 `json:"imageUrl"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// FaceCollection represents a named bucket of enrolled faces.
type FaceCollection struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	FaceCount      int       `json:"faceCount"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// VerifyResult is returned by a 1:1 face verification request.
type VerifyResult struct {
	Match         bool    `json:"match"`
	Confidence    float64 `json:"confidence"`
	ThresholdUsed float64 `json:"thresholdUsed"`
	FaceID        string  `json:"faceId"`
}

// IdentifyMatch is a single match returned by a 1:N identification request.
type IdentifyMatch struct {
	FaceID     string                 `json:"faceId"`
	ExternalID string                 `json:"externalId"`
	Confidence float64                `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// IdentifyResult is returned by a 1:N identification request.
type IdentifyResult struct {
	Matches     []IdentifyMatch `json:"matches"`
	QueryTimeMs int             `json:"queryTimeMs"`
}

// LivenessResult is returned by a passive liveness detection request.
type LivenessResult struct {
	IsLive        bool    `json:"isLive"`
	LivenessScore float64 `json:"livenessScore"`
	FaceDetected  bool    `json:"faceDetected"`
	FaceCount     int     `json:"faceCount"`
}

// BatchFaceResult holds the outcome of a single face in a batch register request.
type BatchFaceResult struct {
	ExternalID string `json:"externalId"`
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

// ─── Face Attributes ─────────────────────────────────────────────────────────

// FaceBBox is a face bounding box in pixel coordinates.
type FaceBBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ImageSize is the analyzed image's dimensions.
type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// HeadPose holds estimated head orientation angles in degrees.
type HeadPose struct {
	Yaw          float64 `json:"yaw"`
	Pitch        float64 `json:"pitch"`
	Roll         float64 `json:"roll"`
	FrontalScore float64 `json:"frontalScore"`
}

// EmotionResult holds emotion classification output.
type EmotionResult struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	Scores     map[string]float64 `json:"scores,omitempty"`
}

// DetectionResult holds a boolean detection (glasses, mask) with confidence.
type DetectionResult struct {
	Detected   bool    `json:"detected"`
	Confidence float64 `json:"confidence"`
}

// FaceAttributes describes one detected face.
type FaceAttributes struct {
	Age          int              `json:"age"`
	Gender       string           `json:"gender"`
	DetScore     float64          `json:"detScore"`
	BBox         FaceBBox         `json:"bbox"`
	Landmarks5pt [][]float64      `json:"landmarks5pt,omitempty"`
	Landmarks106 [][]float64      `json:"landmarks106,omitempty"`
	HeadPose     *HeadPose        `json:"headPose,omitempty"`
	Emotion      *EmotionResult   `json:"emotion,omitempty"`
	Glasses      *DetectionResult `json:"glasses,omitempty"`
	Mask         *DetectionResult `json:"mask,omitempty"`
}

// AttributesResult is the response of the attributes endpoint.
type AttributesResult struct {
	FaceDetected bool             `json:"faceDetected"`
	FaceCount    int              `json:"faceCount"`
	Primary      *FaceAttributes  `json:"primary,omitempty"`
	Faces        []FaceAttributes `json:"faces"`
	ImageSize    *ImageSize       `json:"imageSize,omitempty"`
}

// ─── Async Batch Jobs ────────────────────────────────────────────────────────

// BatchJobResult is the outcome for one image in an async batch job.
type BatchJobResult struct {
	Index      int     `json:"index"`
	ExternalID string  `json:"externalId"`
	FaceID     *string `json:"faceId,omitempty"`
	Error      *string `json:"error,omitempty"`
}

// BatchJob tracks an asynchronous batch registration job.
// Status is one of: queued, processing, done, failed.
type BatchJob struct {
	ID           string           `json:"id"`
	CollectionID string           `json:"collectionId"`
	Status       string           `json:"status"`
	Total        int              `json:"total"`
	Processed    int              `json:"processed"`
	Succeeded    int              `json:"succeeded"`
	Failed       int              `json:"failed"`
	Results      []BatchJobResult `json:"results,omitempty"`
	CreatedAt    string           `json:"createdAt"`
	UpdatedAt    string           `json:"updatedAt"`
}
