// Package livexface provides a Go client for the LiveXFace face recognition API.
package livexface

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.livexface.com/api/v1"

// ─── Error ────────────────────────────────────────────────────────────────────

// APIError is returned when the server responds with a non-successful payload.
type APIError struct {
	Code       string
	Message    string
	StatusCode int
	RequestID  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ─── Client ───────────────────────────────────────────────────────────────────

// Client is the main entry point for the LiveXFace SDK.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Faces provides operations for face enrollment and recognition.
	// Collections are created and managed in the dashboard; the API has no
	// endpoints for that, so the client has no collection operations.
	Faces *FacesResource
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient replaces the underlying *http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// New creates a new Client with the provided API key and optional configuration.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, o := range opts {
		o(c)
	}
	c.Faces = &FacesResource{c: c}
	return c
}

// ─── Internal HTTP helper ─────────────────────────────────────────────────────

// apiEnvelope is the standard response wrapper used by LiveXFace.
type apiEnvelope struct {
	Success   bool            `json:"success"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"requestId"`
	Error     *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// do executes an HTTP request and unwraps the LiveXFace response envelope.
// On a non-successful response it returns a *APIError.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (json.RawMessage, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("livexface: build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("livexface: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("livexface: read body: %w", err)
	}

	// A delete answers 204 with no body. Decoding it used to fail with
	// PARSE_ERROR, so every successful Delete returned an error.
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		// An unknown route answers with a plain-text 404, not the JSON
		// envelope; report the HTTP status rather than a parse failure.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &APIError{
				Code:       fmt.Sprintf("HTTP_%d", resp.StatusCode),
				Message:    fmt.Sprintf("request failed with HTTP %d", resp.StatusCode),
				StatusCode: resp.StatusCode,
			}
		}
		return nil, &APIError{
			Code:       "PARSE_ERROR",
			Message:    "failed to parse response body",
			StatusCode: resp.StatusCode,
		}
	}

	if !env.Success || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			RequestID:  env.RequestID,
		}
		if env.Error != nil {
			apiErr.Code = env.Error.Code
			apiErr.Message = env.Error.Message
		} else {
			apiErr.Code = "UNKNOWN_ERROR"
			apiErr.Message = "an unknown error occurred"
		}
		return nil, apiErr
	}

	return env.Data, nil
}

// doJSON sends a request with a JSON body.
func (c *Client) doJSON(ctx context.Context, method, path string, payload interface{}) (json.RawMessage, error) {
	var bodyReader io.Reader
	contentType := ""
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("livexface: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
		contentType = "application/json"
	}
	return c.do(ctx, method, path, bodyReader, contentType)
}

// ─── Multipart helpers ────────────────────────────────────────────────────────

func imageFilename(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

// writeImagePart adds a file field to a multipart writer with the correct
// content type derived from the filename extension.
func writeImagePart(mw *multipart.Writer, field, filename string, data []byte) error {
	ct := "image/jpeg"
	if len(filename) > 4 {
		switch filename[len(filename)-4:] {
		case ".png":
			ct = "image/png"
		case ".gif":
			ct = "image/gif"
		case "webp":
			ct = "image/webp"
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, filename))
	h.Set("Content-Type", ct)
	pw, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = pw.Write(data)
	return err
}

// ─── FacesResource ────────────────────────────────────────────────────────────

// FacesResource provides face enrollment and recognition operations.
type FacesResource struct {
	c *Client
}

// Register enrolls a new face into a collection.
func (r *FacesResource) Register(ctx context.Context, collectionID string, input RegisterInput) (*Face, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fname := imageFilename(input.Filename, "image.jpg")
	if err := writeImagePart(mw, "image", fname, input.Image); err != nil {
		return nil, fmt.Errorf("livexface: write image: %w", err)
	}
	if err := mw.WriteField("external_id", input.ExternalID); err != nil {
		return nil, err
	}
	if len(input.Metadata) > 0 {
		metaBytes, err := json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("livexface: marshal metadata: %w", err)
		}
		if err := mw.WriteField("metadata", string(metaBytes)); err != nil {
			return nil, err
		}
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/faces",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var face Face
	if err := json.Unmarshal(data, &face); err != nil {
		return nil, fmt.Errorf("livexface: decode face: %w", err)
	}
	return &face, nil
}

// List returns a paginated list of faces in a collection, plus the total count.
func (r *FacesResource) List(ctx context.Context, collectionID string, opts ListOptions) ([]*Face, int, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	path := fmt.Sprintf("/collections/%s/faces?limit=%d&offset=%d", collectionID, limit, opts.Offset)
	data, err := r.c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, 0, err
	}

	var result struct {
		Faces []*Face `json:"faces"`
		Total int     `json:"total"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, fmt.Errorf("livexface: decode faces list: %w", err)
	}
	return result.Faces, result.Total, nil
}

// Get retrieves a face by its ID.
func (r *FacesResource) Get(ctx context.Context, collectionID, faceID string) (*Face, error) {
	data, err := r.c.do(ctx, http.MethodGet,
		"/collections/"+collectionID+"/faces/"+faceID, nil, "")
	if err != nil {
		return nil, err
	}
	var face Face
	if err := json.Unmarshal(data, &face); err != nil {
		return nil, fmt.Errorf("livexface: decode face: %w", err)
	}
	return &face, nil
}

// Delete removes a face from a collection.
func (r *FacesResource) Delete(ctx context.Context, collectionID, faceID string) error {
	_, err := r.c.do(ctx, http.MethodDelete,
		"/collections/"+collectionID+"/faces/"+faceID, nil, "")
	return err
}

// Verify performs a 1:1 face verification against a stored face.
func (r *FacesResource) Verify(ctx context.Context, collectionID string, input VerifyInput) (*VerifyResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fname := imageFilename(input.Filename, "image.jpg")
	if err := writeImagePart(mw, "image", fname, input.Image); err != nil {
		return nil, fmt.Errorf("livexface: write image: %w", err)
	}
	if input.FaceID != "" {
		if err := mw.WriteField("face_id", input.FaceID); err != nil {
			return nil, err
		}
	}
	if input.Threshold > 0 {
		if err := mw.WriteField("threshold", strconv.FormatFloat(input.Threshold, 'f', -1, 64)); err != nil {
			return nil, err
		}
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/verify",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result VerifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("livexface: decode verify result: %w", err)
	}
	return &result, nil
}

// Identify performs a 1:N face identification against a collection.
func (r *FacesResource) Identify(ctx context.Context, collectionID string, input IdentifyInput) (*IdentifyResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fname := imageFilename(input.Filename, "image.jpg")
	if err := writeImagePart(mw, "image", fname, input.Image); err != nil {
		return nil, fmt.Errorf("livexface: write image: %w", err)
	}
	topK := input.TopK
	if topK == 0 {
		topK = 5
	}
	if err := mw.WriteField("top_k", strconv.Itoa(topK)); err != nil {
		return nil, err
	}
	if input.Threshold > 0 {
		if err := mw.WriteField("threshold", strconv.FormatFloat(input.Threshold, 'f', -1, 64)); err != nil {
			return nil, err
		}
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/identify",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result IdentifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("livexface: decode identify result: %w", err)
	}
	return &result, nil
}

// Liveness runs a passive liveness detection check on the provided image.
func (r *FacesResource) Liveness(ctx context.Context, collectionID string, image []byte, filename string) (*LivenessResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fname := imageFilename(filename, "image.jpg")
	if err := writeImagePart(mw, "image", fname, image); err != nil {
		return nil, fmt.Errorf("livexface: write image: %w", err)
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/liveness",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result LivenessResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("livexface: decode liveness result: %w", err)
	}
	return &result, nil
}

// Compare performs a pairwise comparison of two face images without enrolling
// either into a collection.
func (r *FacesResource) Compare(ctx context.Context, input CompareInput) (*VerifyResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	f1 := imageFilename(input.Filename1, "image1.jpg")
	if err := writeImagePart(mw, "image1", f1, input.Image1); err != nil {
		return nil, fmt.Errorf("livexface: write image1: %w", err)
	}
	f2 := imageFilename(input.Filename2, "image2.jpg")
	if err := writeImagePart(mw, "image2", f2, input.Image2); err != nil {
		return nil, fmt.Errorf("livexface: write image2: %w", err)
	}
	if input.Threshold > 0 {
		if err := mw.WriteField("threshold", strconv.FormatFloat(input.Threshold, 'f', -1, 64)); err != nil {
			return nil, err
		}
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost, "/compare", &buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result VerifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("livexface: decode compare result: %w", err)
	}
	return &result, nil
}

// BatchRegister enrolls up to 20 faces in a single request.
func (r *FacesResource) BatchRegister(ctx context.Context, collectionID string, items []BatchItem) (*BatchResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	type entry struct {
		ExternalID string                 `json:"externalId"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	entries := make([]entry, 0, len(items))

	for i, item := range items {
		field := fmt.Sprintf("images[%d]", i)
		fname := imageFilename(item.Filename, "image.jpg")
		if err := writeImagePart(mw, field, fname, item.Image); err != nil {
			return nil, fmt.Errorf("livexface: write image[%d]: %w", i, err)
		}
		meta := item.Metadata
		if meta == nil {
			meta = map[string]interface{}{}
		}
		entries = append(entries, entry{ExternalID: item.ExternalID, Metadata: meta})
	}

	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("livexface: marshal entries: %w", err)
	}
	if err := mw.WriteField("entries", string(entriesJSON)); err != nil {
		return nil, err
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/faces/batch",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result BatchResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("livexface: decode batch response: %w", err)
	}
	return &result, nil
}

// Attributes detects face attributes (age, gender, emotion, glasses, mask,
// head pose, landmarks) for all faces in an image. No face is enrolled.
func (r *FacesResource) Attributes(ctx context.Context, collectionID string, image []byte, filename string) (*AttributesResult, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := writeImagePart(mw, "image", imageFilename(filename, "image.jpg"), image); err != nil {
		return nil, err
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/attributes",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var result AttributesResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("livexface: decode attributes response: %w", err)
	}
	return &result, nil
}

// BatchRegisterAsync submits up to 100 faces for asynchronous registration.
// It returns the created job immediately; poll GetBatchJob until the job's
// Status is "done" or "failed".
func (r *FacesResource) BatchRegisterAsync(ctx context.Context, collectionID string, items []BatchItem) (*BatchJob, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	type entry struct {
		ExternalID string                 `json:"externalId"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	entries := make([]entry, 0, len(items))

	for i, item := range items {
		field := fmt.Sprintf("images[%d]", i)
		fname := imageFilename(item.Filename, "image.jpg")
		if err := writeImagePart(mw, field, fname, item.Image); err != nil {
			return nil, fmt.Errorf("livexface: write image[%d]: %w", i, err)
		}
		meta := item.Metadata
		if meta == nil {
			meta = map[string]interface{}{}
		}
		entries = append(entries, entry{ExternalID: item.ExternalID, Metadata: meta})
	}

	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("livexface: marshal entries: %w", err)
	}
	if err := mw.WriteField("entries", string(entriesJSON)); err != nil {
		return nil, err
	}
	mw.Close()

	data, err := r.c.do(ctx, http.MethodPost,
		"/collections/"+collectionID+"/faces/batch-async",
		&buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var job BatchJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("livexface: decode batch job: %w", err)
	}
	return &job, nil
}

// GetBatchJob fetches the status (and, when available, per-image results) of
// an async batch registration job.
func (r *FacesResource) GetBatchJob(ctx context.Context, collectionID, jobID string) (*BatchJob, error) {
	data, err := r.c.do(ctx, http.MethodGet,
		"/collections/"+collectionID+"/batch/"+jobID, nil, "")
	if err != nil {
		return nil, err
	}
	var job BatchJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("livexface: decode batch job: %w", err)
	}
	return &job, nil
}
