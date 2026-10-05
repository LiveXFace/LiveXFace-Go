package livexface

import (
	"encoding/json"
	"time"
)

// Face represents a registered face in a collection.
type Face struct {
	ID           string                 `json:"id"`
	CollectionID string                 `json:"collectionId"`
	ExternalID   string                 `json:"externalId"`
	Metadata     map[string]interface{} `json:"metadata"`
	ImageURL     string                 `json:"imageUrl"`
	CreatedAt    time.Time              `json:"createdAt"`
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

// CrossCollectionSearchMatch is a match returned by a cross-collection search.
type CrossCollectionSearchMatch struct {
	FaceID       string                 `json:"faceId"`
	ExternalID   string                 `json:"externalId"`
	CollectionID string                 `json:"collectionId"`
	Confidence   float64                `json:"confidence"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// SkippedCollection explains why a collection was excluded from a search.
type SkippedCollection struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// CrossCollectionSearchResult is returned by Search.
type CrossCollectionSearchResult struct {
	Matches             []CrossCollectionSearchMatch `json:"matches"`
	QueryTimeMs         int                          `json:"queryTimeMs"`
	CollectionsSearched int                          `json:"collectionsSearched"`
	SkippedCollections  []SkippedCollection          `json:"skippedCollections"`
}

// LivenessResult is returned by a passive liveness detection request.
type LivenessResult struct {
	IsLive        bool    `json:"isLive"`
	LivenessScore float64 `json:"livenessScore"`
	FaceDetected  bool    `json:"faceDetected"`
	FaceCount     int     `json:"faceCount"`
}

// LivenessFrame is a single frame (JPEG or PNG) submitted to ActiveLiveness
// or CompleteLivenessSession.
type LivenessFrame struct {
	Image []byte
	// Filename is optional; defaults to "frame_<n>.jpg".
	Filename string
}

// LivenessChallenge is the outcome of one active liveness challenge.
// Passed is nil when the challenge could not be evaluated. Any additional
// metric keys returned by the server are kept in Metrics.
type LivenessChallenge struct {
	Passed    *bool                  `json:"passed"`
	Available bool                   `json:"available"`
	Metrics   map[string]interface{} `json:"-"`
}

// UnmarshalJSON decodes passed/available and keeps every other key in Metrics.
func (c *LivenessChallenge) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*c = LivenessChallenge{}
	if v, ok := raw["passed"]; ok {
		if err := json.Unmarshal(v, &c.Passed); err != nil {
			return err
		}
		delete(raw, "passed")
	}
	if v, ok := raw["available"]; ok {
		if err := json.Unmarshal(v, &c.Available); err != nil {
			return err
		}
		delete(raw, "available")
	}
	if len(raw) > 0 {
		c.Metrics = make(map[string]interface{}, len(raw))
		for k, v := range raw {
			var val interface{}
			if err := json.Unmarshal(v, &val); err != nil {
				return err
			}
			c.Metrics[k] = val
		}
	}
	return nil
}

// LivenessChallenges groups the per-challenge results of an active liveness check.
type LivenessChallenges struct {
	Blink            LivenessChallenge `json:"blink"`
	HeadTurn         LivenessChallenge `json:"headTurn"`
	PassiveAntispoof LivenessChallenge `json:"passiveAntispoof"`
}

// ActiveLivenessResult is returned by an active (multi-frame) liveness check.
// It is a verdict only: liveness tokens come from CompleteLivenessSession.
type ActiveLivenessResult struct {
	IsLive         bool               `json:"isLive"`
	OverallScore   float64            `json:"overallScore"`
	FramesAnalyzed int                `json:"framesAnalyzed"`
	FramesWithFace int                `json:"framesWithFace"`
	Challenges     LivenessChallenges `json:"challenges"`
}

// LivenessStepType is one step a liveness session asks the person to perform.
// Turns are in the person's own left and right.
type LivenessStepType string

// The step types a liveness session can ask for.
const (
	LivenessStepBlink     LivenessStepType = "blink"
	LivenessStepTurnLeft  LivenessStepType = "turn_left"
	LivenessStepTurnRight LivenessStepType = "turn_right"
)

// LivenessSessionChallenge is one step of a liveness session.
type LivenessSessionChallenge struct {
	Type LivenessStepType `json:"type"`
}

// LivenessSession is returned by CreateLivenessSession. Show its Challenges
// to the person in order, capture frames while they perform them and submit
// the frames with CompleteLivenessSession before ExpiresAt.
type LivenessSession struct {
	SessionID  string                     `json:"sessionId"`
	Challenges []LivenessSessionChallenge `json:"challenges"`
	ExpiresAt  time.Time                  `json:"expiresAt"`
}

// LivenessStep reports whether one session step was performed, in its turn.
type LivenessStep struct {
	Type   LivenessStepType `json:"type"`
	Passed bool             `json:"passed"`
}

// LivenessSessionResult is returned by CompleteLivenessSession: the active
// liveness verdict plus the session's steps in order. LivenessToken and
// LivenessTokenExpiresAt are set only when the session passed (and the server
// could store the token).
type LivenessSessionResult struct {
	ActiveLivenessResult
	Steps                  []LivenessStep `json:"steps"`
	LivenessToken          string         `json:"livenessToken,omitempty"`
	LivenessTokenExpiresAt *time.Time     `json:"livenessTokenExpiresAt,omitempty"`
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
	// LivenessToken is optional; a token from a passed liveness session
	// (CompleteLivenessSession). Required when the collection requires liveness.
	LivenessToken string
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

// CrossCollectionSearchInput holds parameters for a search across collections.
type CrossCollectionSearchInput struct {
	Image         []byte
	Filename      string
	CollectionIDs []string
	TopK          int
	Threshold     float64
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

// BatchItem is a single entry in a batch register request.
type BatchItem struct {
	ExternalID string
	Image      []byte
	Filename   string
	Metadata   map[string]interface{}
	// LivenessToken is optional; a token from a passed liveness session
	// (CompleteLivenessSession). Required when the collection requires liveness.
	LivenessToken string
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
