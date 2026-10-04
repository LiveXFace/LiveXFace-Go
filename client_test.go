package livexface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type capturedRequest struct {
	Method string
	Path   string
	Files  map[string]int
	Fields map[string]string
}

func newTestServer(t *testing.T, data string) (*Client, *capturedRequest) {
	t.Helper()
	got := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.Path
		if err := r.ParseMultipartForm(32 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
			t.Errorf("parse multipart: %v", err)
		}
		got.Files = map[string]int{}
		got.Fields = map[string]string{}
		if r.MultipartForm != nil {
			for k, fhs := range r.MultipartForm.File {
				got.Files[k] = len(fhs)
			}
			for k, v := range r.MultipartForm.Value {
				got.Fields[k] = v[0]
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":%s}`, data)
	}))
	t.Cleanup(srv.Close)
	return New("lxf_test", WithBaseURL(srv.URL+"/api/v1")), got
}

func frames(n int) []LivenessFrame {
	out := make([]LivenessFrame, n)
	for i := range out {
		out[i] = LivenessFrame{Image: []byte{0xff, 0xd8, byte(i)}}
	}
	return out
}

func TestActiveLivenessPassed(t *testing.T) {
	c, got := newTestServer(t, `{
		"isLive": true, "overallScore": 0.93, "framesAnalyzed": 6, "framesWithFace": 6,
		"challenges": {
			"blink": {"passed": true, "available": true, "blinkCount": 2},
			"headTurn": {"passed": null, "available": false},
			"passiveAntispoof": {"passed": true, "available": true, "score": 0.97}
		}
	}`)

	res, err := c.Faces.ActiveLiveness(context.Background(), "col_1", frames(6))
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/collections/col_1/active-liveness" {
		t.Errorf("request = %s %s", got.Method, got.Path)
	}
	for i := 0; i < 6; i++ {
		if got.Files[fmt.Sprintf("frame_%d", i)] != 1 {
			t.Errorf("missing part frame_%d (files: %v)", i, got.Files)
		}
	}
	if len(got.Files) != 6 {
		t.Errorf("got %d file parts, want 6", len(got.Files))
	}
	if !res.IsLive || res.FramesWithFace != 6 {
		t.Errorf("unexpected result: %+v", res)
	}
	if b := res.Challenges.Blink; b.Passed == nil || !*b.Passed || !b.Available || b.Metrics["blinkCount"] != float64(2) {
		t.Errorf("blink = %+v", b)
	}
	if h := res.Challenges.HeadTurn; h.Passed != nil || h.Available {
		t.Errorf("headTurn = %+v", h)
	}
}

func TestActiveLivenessFailed(t *testing.T) {
	c, _ := newTestServer(t, `{
		"isLive": false, "overallScore": 0.21, "framesAnalyzed": 5, "framesWithFace": 5,
		"challenges": {
			"blink": {"passed": false, "available": true},
			"headTurn": {"passed": false, "available": true},
			"passiveAntispoof": {"passed": true, "available": true}
		}
	}`)

	res, err := c.Faces.ActiveLiveness(context.Background(), "col_1", frames(5))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsLive {
		t.Errorf("unexpected result: %+v", res)
	}
	if p := res.Challenges.Blink.Passed; p == nil || *p {
		t.Errorf("blink.passed = %v, want false", p)
	}
}

// The stateless check issues no token since contract 2.0.0; only a completed
// liveness session does.
func TestActiveLivenessResultHasNoToken(t *testing.T) {
	typ := reflect.TypeOf(ActiveLivenessResult{})
	for i := 0; i < typ.NumField(); i++ {
		if f := typ.Field(i); strings.Contains(f.Name, "Token") || strings.Contains(f.Tag.Get("json"), "Token") {
			t.Errorf("ActiveLivenessResult has token field %s", f.Name)
		}
	}
}

// ─── Liveness sessions ────────────────────────────────────────────────────────

func TestCreateLivenessSession(t *testing.T) {
	c, got := newTestServer(t, `{
		"sessionId": "lvs_abc",
		"challenges": [{"type": "turn_left"}, {"type": "blink"}, {"type": "turn_right"}],
		"expiresAt": "2026-10-03T10:01:00Z"
	}`)

	s, err := c.Faces.CreateLivenessSession(context.Background(), "col_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/collections/col_1/liveness-sessions" {
		t.Errorf("request = %s %s", got.Method, got.Path)
	}
	if len(got.Files)+len(got.Fields) != 0 {
		t.Errorf("sent a body: files %v, fields %v", got.Files, got.Fields)
	}
	want := []LivenessStepType{LivenessStepTurnLeft, LivenessStepBlink, LivenessStepTurnRight}
	if s.SessionID != "lvs_abc" || len(s.Challenges) != len(want) {
		t.Fatalf("session = %+v", s)
	}
	for i, ch := range s.Challenges {
		if ch.Type != want[i] {
			t.Errorf("challenge %d = %q, want %q", i, ch.Type, want[i])
		}
	}
	if !s.ExpiresAt.Equal(time.Date(2026, 10, 3, 10, 1, 0, 0, time.UTC)) {
		t.Errorf("expiresAt = %v", s.ExpiresAt)
	}
}

func TestCompleteLivenessSessionPassed(t *testing.T) {
	c, got := newTestServer(t, `{
		"isLive": true, "overallScore": 0.91, "framesAnalyzed": 25, "framesWithFace": 25,
		"challenges": {
			"blink": {"passed": true, "available": true},
			"headTurn": {"passed": true, "available": true},
			"passiveAntispoof": {"passed": true, "available": true, "score": 0.95}
		},
		"steps": [{"type": "turn_left", "passed": true}, {"type": "blink", "passed": true}],
		"livenessToken": "lvt_abc",
		"livenessTokenExpiresAt": "2026-10-03T10:06:00Z"
	}`)

	res, err := c.Faces.CompleteLivenessSession(context.Background(), "col_1", "lvs_abc", frames(25), true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/collections/col_1/liveness-sessions/lvs_abc" {
		t.Errorf("request = %s %s", got.Method, got.Path)
	}
	for i := 0; i < 25; i++ {
		if got.Files[fmt.Sprintf("frame_%d", i)] != 1 {
			t.Errorf("missing part frame_%d (files: %v)", i, got.Files)
		}
	}
	if len(got.Files) != 25 || got.Fields["mirrored"] != "true" {
		t.Errorf("files %d, mirrored %q; want 25 and true", len(got.Files), got.Fields["mirrored"])
	}
	if !res.IsLive || res.FramesAnalyzed != 25 || res.LivenessToken != "lvt_abc" {
		t.Errorf("unexpected result: %+v", res)
	}
	if want := time.Date(2026, 10, 3, 10, 6, 0, 0, time.UTC); res.LivenessTokenExpiresAt == nil || !res.LivenessTokenExpiresAt.Equal(want) {
		t.Errorf("token expiresAt = %v, want %v", res.LivenessTokenExpiresAt, want)
	}
	wantSteps := []LivenessStep{{LivenessStepTurnLeft, true}, {LivenessStepBlink, true}}
	if !reflect.DeepEqual(res.Steps, wantSteps) {
		t.Errorf("steps = %+v, want %+v", res.Steps, wantSteps)
	}
	if p := res.Challenges.HeadTurn.Passed; p == nil || !*p {
		t.Errorf("headTurn.passed = %v, want true", p)
	}
}

func TestCompleteLivenessSessionFailedHasNoToken(t *testing.T) {
	c, got := newTestServer(t, `{
		"isLive": false, "overallScore": 0.4, "framesAnalyzed": 20, "framesWithFace": 20,
		"challenges": {
			"blink": {"passed": true, "available": true},
			"headTurn": {"passed": true, "available": true},
			"passiveAntispoof": {"passed": true, "available": true}
		},
		"steps": [{"type": "blink", "passed": true}, {"type": "turn_right", "passed": false}]
	}`)

	res, err := c.Faces.CompleteLivenessSession(context.Background(), "col_1", "lvs_abc", frames(20), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fields["mirrored"] != "false" {
		t.Errorf("mirrored = %q, want false", got.Fields["mirrored"])
	}
	if res.IsLive || res.LivenessToken != "" || res.LivenessTokenExpiresAt != nil {
		t.Errorf("unexpected result: %+v", res)
	}
	if len(res.Steps) != 2 || res.Steps[1] != (LivenessStep{LivenessStepTurnRight, false}) {
		t.Errorf("steps = %+v", res.Steps)
	}
}

func TestCompleteLivenessSessionInvalid(t *testing.T) {
	c, s := scripted(t, []Option{WithRetries(2)},
		reply(http.StatusUnprocessableEntity, apiError(422, "LIVENESS_SESSION_INVALID")))

	_, err := c.Faces.CompleteLivenessSession(context.Background(), "col_1", "lvs_used", frames(5), false)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Code != "LIVENESS_SESSION_INVALID" || apiErr.StatusCode != 422 || apiErr.RequestID != "req_422" {
		t.Errorf("error = %+v", apiErr)
	}
	if n := len(s.requests()); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
}

// A retry would find the session used up, so completion is not repeated after
// a dropped connection or a 503 (the server consumes the session first), but
// is after a 429, which the server sends before touching the session.
func TestCompleteLivenessSessionRetries(t *testing.T) {
	complete := func(c *Client) error {
		_, err := c.Faces.CompleteLivenessSession(context.Background(), "col_1", "lvs_abc", frames(5), false)
		return err
	}
	t.Run("not retried on a dropped connection", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(3)}, drop)
		var apiErr *APIError
		if err := complete(c); err == nil || errors.As(err, &apiErr) {
			t.Fatalf("want a network error, got %v", err)
		}
		if keys := s.requests(); len(keys) != 1 || keys[0] != "" {
			t.Errorf("requests = %q, want one without a key", keys)
		}
	})
	t.Run("not retried on 503", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(3)},
			reply(http.StatusServiceUnavailable, apiError(503, "SERVICE_BUSY"), "Retry-After", "1"))
		var apiErr *APIError
		if err := complete(c); !errors.As(err, &apiErr) || apiErr.Code != "SERVICE_BUSY" {
			t.Fatalf("want SERVICE_BUSY, got %v", err)
		}
		if n := len(s.requests()); n != 1 {
			t.Errorf("%d requests, want 1", n)
		}
	})
	t.Run("retried on 429", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(3)},
			reply(http.StatusTooManyRequests, apiError(429, "RATE_LIMIT_EXCEEDED"), "Retry-After", "1"),
			reply(http.StatusOK, `{"success":true,"data":{"isLive":false,"steps":[]}}`))
		if err := complete(c); err != nil {
			t.Fatal(err)
		}
		if n := len(s.requests()); n != 2 {
			t.Errorf("%d requests, want 2", n)
		}
	})
}

func TestRegisterLivenessToken(t *testing.T) {
	face := `{"id":"face_1","externalId":"u1"}`
	for _, tc := range []struct {
		name, token string
	}{{"with token", "lvt_abc"}, {"without token", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			c, got := newTestServer(t, face)
			_, err := c.Faces.Register(context.Background(), "col_1", RegisterInput{
				ExternalID: "u1", Image: []byte{0xff, 0xd8}, LivenessToken: tc.token,
			})
			if err != nil {
				t.Fatal(err)
			}
			v, ok := got.Fields["liveness_token"]
			if tc.token == "" && ok {
				t.Errorf("liveness_token sent when unset: %q", v)
			}
			if tc.token != "" && v != tc.token {
				t.Errorf("liveness_token = %q, want %q", v, tc.token)
			}
		})
	}
}

func TestBatchEntriesLivenessToken(t *testing.T) {
	items := []BatchItem{
		{ExternalID: "a", Image: []byte{1}, LivenessToken: "lvt_a"},
		{ExternalID: "b", Image: []byte{2}},
	}
	check := func(t *testing.T, raw string) {
		t.Helper()
		var entries []map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			t.Fatalf("entries: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries", len(entries))
		}
		if entries[0]["livenessToken"] != "lvt_a" || entries[0]["externalId"] != "a" {
			t.Errorf("entry 0 = %v", entries[0])
		}
		if _, ok := entries[1]["livenessToken"]; ok {
			t.Errorf("entry 1 has livenessToken: %v", entries[1])
		}
	}

	t.Run("sync", func(t *testing.T) {
		c, got := newTestServer(t, `{"succeeded":2,"failed":0,"results":[]}`)
		if _, err := c.Faces.BatchRegister(context.Background(), "col_1", items); err != nil {
			t.Fatal(err)
		}
		if got.Path != "/api/v1/collections/col_1/faces/batch" {
			t.Errorf("path = %s", got.Path)
		}
		check(t, got.Fields["entries"])
	})
	t.Run("async", func(t *testing.T) {
		c, got := newTestServer(t, `{"id":"job_1","status":"queued"}`)
		if _, err := c.Faces.BatchRegisterAsync(context.Background(), "col_1", items); err != nil {
			t.Fatal(err)
		}
		if got.Path != "/api/v1/collections/col_1/faces/batch-async" {
			t.Errorf("path = %s", got.Path)
		}
		check(t, got.Fields["entries"])
	})
}

func TestAPIErrorExposesDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"success":false,"requestId":"r-1","error":{"code":"MULTIPLE_FACES","message":"multiple faces detected","details":{"faceCount":2,"faces":[]}}}`))
	}))
	defer srv.Close()

	c := New("lxf_test_key", WithBaseURL(srv.URL))
	_, err := c.Faces.Register(context.Background(), "col", RegisterInput{Image: []byte("img"), Filename: "a.jpg"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.Code != "MULTIPLE_FACES" || apiErr.Details["faceCount"] != float64(2) {
		t.Fatalf("details not exposed: %+v", apiErr)
	}
	if apiErr.RetryAfter != nil {
		t.Errorf("RetryAfter = %d without a Retry-After header", *apiErr.RetryAfter)
	}
}

// ─── Idempotency keys and retries ─────────────────────────────────────────────

const faceJSON = `{"success":true,"data":{"id":"face_1","externalId":"u1"}}`

// reply answers with status, body and header name/value pairs.
func reply(status int, body string, header ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		for i := 0; i+1 < len(header); i += 2 {
			w.Header().Set(header[i], header[i+1])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// drop reads the request, then closes the connection without a response.
func drop(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		panic(err)
	}
	conn.Close()
}

func apiError(status int, code string) string {
	return fmt.Sprintf(`{"success":false,"requestId":"req_%d","error":{"code":%q,"message":"x"}}`, status, code)
}

type script struct {
	mu    sync.Mutex
	keys  []string        // Idempotency-Key of each request, "" when absent
	slept []time.Duration // delays passed to the client's sleep
}

func (s *script) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.keys...)
}

// scripted answers the n-th request with steps[n] and records each request's
// Idempotency-Key. The client's sleep is replaced so retries do not wait.
func scripted(t *testing.T, opts []Option, steps ...http.HandlerFunc) (*Client, *script) {
	t.Helper()
	s := &script{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		n := len(s.keys)
		s.keys = append(s.keys, r.Header.Get("Idempotency-Key"))
		s.mu.Unlock()
		if n >= len(steps) {
			t.Errorf("unexpected request %d", n+1)
			reply(http.StatusInternalServerError, apiError(500, "INTERNAL_ERROR"))(w, r)
			return
		}
		steps[n](w, r)
	}))
	t.Cleanup(srv.Close)
	c := New("lxf_test", append([]Option{WithBaseURL(srv.URL)}, opts...)...)
	c.sleep = func(_ context.Context, d time.Duration) error {
		s.slept = append(s.slept, d)
		return nil
	}
	return c, s
}

var registerInput = RegisterInput{ExternalID: "u1", Image: []byte{0xff, 0xd8}}

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestRateLimitedExposesRetryAfter(t *testing.T) {
	c, s := scripted(t, nil,
		reply(http.StatusTooManyRequests, apiError(429, "RATE_LIMIT_EXCEEDED"), "Retry-After", "12"))

	_, err := c.Faces.Identify(context.Background(), "col_1", IdentifyInput{Image: []byte{1}})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %v", err)
	}
	if apiErr.StatusCode != 429 || apiErr.Code != "RATE_LIMIT_EXCEEDED" || apiErr.RequestID != "req_429" {
		t.Errorf("error = %+v", apiErr)
	}
	if apiErr.RetryAfter == nil || *apiErr.RetryAfter != 12 {
		t.Errorf("RetryAfter = %v, want 12", apiErr.RetryAfter)
	}
	if n := len(s.requests()); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
}

func TestRetryBusyEngineWithGeneratedKey(t *testing.T) {
	c, s := scripted(t, []Option{WithRetries(2)},
		reply(http.StatusServiceUnavailable, apiError(503, "SERVICE_BUSY"), "Retry-After", "5"),
		reply(http.StatusCreated, faceJSON))

	face, err := c.Faces.Register(context.Background(), "col_1", registerInput)
	if err != nil {
		t.Fatal(err)
	}
	if face.ID != "face_1" {
		t.Errorf("face = %+v", face)
	}
	keys := s.requests()
	if len(keys) != 2 || !uuidV4.MatchString(keys[0]) || keys[1] != keys[0] {
		t.Errorf("keys = %q, want one generated key sent twice", keys)
	}
	if len(s.slept) != 1 || s.slept[0] != 5*time.Second {
		t.Errorf("slept %v, want [5s]", s.slept)
	}
}

func TestRetryDroppedConnectionWithCallerKey(t *testing.T) {
	c, s := scripted(t, []Option{WithRetries(2)},
		drop,
		reply(http.StatusCreated, faceJSON, "Idempotent-Replayed", "true"))

	face, err := c.Faces.Register(context.Background(), "col_1", registerInput, WithIdempotencyKey("enrol-u1"))
	if err != nil {
		t.Fatal(err)
	}
	if face.ID != "face_1" {
		t.Errorf("face = %+v", face)
	}
	if keys := s.requests(); len(keys) != 2 || keys[0] != "enrol-u1" || keys[1] != "enrol-u1" {
		t.Errorf("keys = %q, want the caller's key on both attempts", keys)
	}
}

func TestNoRetryOnValidationError(t *testing.T) {
	c, s := scripted(t, []Option{WithRetries(3)},
		reply(http.StatusUnprocessableEntity, apiError(422, "NO_FACE_DETECTED")))

	_, err := c.Faces.Register(context.Background(), "col_1", registerInput)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "NO_FACE_DETECTED" {
		t.Fatalf("want NO_FACE_DETECTED, got %v", err)
	}
	if n := len(s.requests()); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
}

func TestRetriesStopAndSurfaceLastError(t *testing.T) {
	busy := reply(http.StatusServiceUnavailable, apiError(503, "SERVICE_BUSY"))
	c, s := scripted(t, []Option{WithRetries(2), WithMaxRetryDelay(time.Second)}, busy, busy, busy)

	_, err := c.Faces.Register(context.Background(), "col_1", registerInput)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 503 {
		t.Fatalf("want the 503, got %v", err)
	}
	if n := len(s.requests()); n != 3 {
		t.Errorf("%d requests, want 3", n)
	}
	for _, d := range s.slept {
		if d < 0 || d > time.Second {
			t.Errorf("backoff %v outside [0, 1s]", d)
		}
	}
}

func TestIdempotencyKeyHeader(t *testing.T) {
	ok := map[string]string{
		"register": faceJSON,
		"batch":    `{"success":true,"data":{"succeeded":1,"failed":0,"results":[]}}`,
		"async":    `{"success":true,"data":{"id":"job_1","status":"queued"}}`,
	}
	call := func(c *Client, name string, opts ...CallOption) error {
		ctx, items := context.Background(), []BatchItem{{ExternalID: "a", Image: []byte{1}}}
		var err error
		switch name {
		case "register":
			_, err = c.Faces.Register(ctx, "col_1", registerInput, opts...)
		case "batch":
			_, err = c.Faces.BatchRegister(ctx, "col_1", items, opts...)
		case "async":
			_, err = c.Faces.BatchRegisterAsync(ctx, "col_1", items, opts...)
		}
		return err
	}
	for name, body := range ok {
		t.Run(name+" caller key", func(t *testing.T) {
			c, s := scripted(t, nil, reply(http.StatusCreated, body))
			if err := call(c, name, WithIdempotencyKey("k-123")); err != nil {
				t.Fatal(err)
			}
			if keys := s.requests(); keys[0] != "k-123" {
				t.Errorf("Idempotency-Key = %q, want k-123", keys[0])
			}
		})
		t.Run(name+" no key, retries off", func(t *testing.T) {
			c, s := scripted(t, nil, reply(http.StatusCreated, body))
			if err := call(c, name); err != nil {
				t.Fatal(err)
			}
			if keys := s.requests(); keys[0] != "" {
				t.Errorf("Idempotency-Key = %q, want none", keys[0])
			}
		})
	}
}

func TestNewIdempotencyKey(t *testing.T) {
	a, b := NewIdempotencyKey(), NewIdempotencyKey()
	if !uuidV4.MatchString(a) || !uuidV4.MatchString(b) || a == b {
		t.Errorf("keys %q and %q, want two different UUID v4s", a, b)
	}
}

func TestRetryPolicyForPlainRequests(t *testing.T) {
	identify := func(c *Client) error {
		_, err := c.Faces.Identify(context.Background(), "col_1", IdentifyInput{Image: []byte{1}})
		return err
	}
	matches := reply(http.StatusOK, `{"success":true,"data":{"matches":[]}}`)

	t.Run("POST not retried on a dropped connection", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(2)}, drop)
		var apiErr *APIError
		if err := identify(c); err == nil || errors.As(err, &apiErr) {
			t.Fatalf("want a network error, got %v", err)
		}
		if keys := s.requests(); len(keys) != 1 || keys[0] != "" {
			t.Errorf("requests = %q, want one without a key", keys)
		}
	})
	t.Run("POST not retried on 500", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(2)},
			reply(http.StatusInternalServerError, apiError(500, "INTERNAL_ERROR")))
		if err := identify(c); err == nil {
			t.Fatal("want an error")
		}
		if n := len(s.requests()); n != 1 {
			t.Errorf("%d requests, want 1", n)
		}
	})
	t.Run("POST retried on 503", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(2)},
			reply(http.StatusServiceUnavailable, apiError(503, "SERVICE_BUSY")), matches)
		if err := identify(c); err != nil {
			t.Fatal(err)
		}
		if n := len(s.requests()); n != 2 {
			t.Errorf("%d requests, want 2", n)
		}
	})
	t.Run("GET retried on a dropped connection", func(t *testing.T) {
		c, s := scripted(t, []Option{WithRetries(2)}, drop, reply(http.StatusOK, faceJSON))
		if _, err := c.Faces.Get(context.Background(), "col_1", "face_1"); err != nil {
			t.Fatal(err)
		}
		if n := len(s.requests()); n != 2 {
			t.Errorf("%d requests, want 2", n)
		}
	})
}
