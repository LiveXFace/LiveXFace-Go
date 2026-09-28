package livexface

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		if err := r.ParseMultipartForm(32 << 20); err != nil {
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
		},
		"livenessToken": "lvt_abc",
		"livenessTokenExpiresAt": "2026-09-28T10:05:00Z"
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
	if !res.IsLive || res.LivenessToken != "lvt_abc" || res.FramesWithFace != 6 {
		t.Errorf("unexpected result: %+v", res)
	}
	want := time.Date(2026, 9, 28, 10, 5, 0, 0, time.UTC)
	if res.LivenessTokenExpiresAt == nil || !res.LivenessTokenExpiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v", res.LivenessTokenExpiresAt, want)
	}
	if b := res.Challenges.Blink; b.Passed == nil || !*b.Passed || !b.Available || b.Metrics["blinkCount"] != float64(2) {
		t.Errorf("blink = %+v", b)
	}
	if h := res.Challenges.HeadTurn; h.Passed != nil || h.Available {
		t.Errorf("headTurn = %+v", h)
	}
}

func TestActiveLivenessFailedHasNoToken(t *testing.T) {
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
	if res.IsLive || res.LivenessToken != "" || res.LivenessTokenExpiresAt != nil {
		t.Errorf("unexpected result: %+v", res)
	}
	if p := res.Challenges.Blink.Passed; p == nil || *p {
		t.Errorf("blink.passed = %v, want false", p)
	}
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
