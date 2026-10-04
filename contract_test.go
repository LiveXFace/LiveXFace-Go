package livexface

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The contract check: every public SDK method is called against a fake
// server, and each request it makes must use a method and path present in the
// pinned contract (contract/openapi-<ContractVersion>.json) and send every
// field the contract marks required.

type contractSchema struct {
	Ref        string                     `json:"$ref"`
	Required   []string                   `json:"required"`
	Properties map[string]json.RawMessage `json:"properties"`
}

type contractOperation struct {
	Parameters []struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required bool   `json:"required"`
	} `json:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema contractSchema `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

type contractDoc struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Paths      map[string]map[string]contractOperation `json:"paths"`
	Components struct {
		Schemas map[string]contractSchema `json:"schemas"`
	} `json:"components"`
}

func loadContract(t *testing.T) *contractDoc {
	t.Helper()
	raw, err := os.ReadFile("contract/openapi-" + ContractVersion + ".json")
	if err != nil {
		t.Fatalf("read pinned contract: %v", err)
	}
	var doc contractDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse pinned contract: %v", err)
	}
	return &doc
}

func TestContractVersion(t *testing.T) {
	raw, err := os.ReadFile("CONTRACT_VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if v := strings.TrimSpace(string(raw)); v != ContractVersion {
		t.Errorf("CONTRACT_VERSION file = %q, ContractVersion = %q", v, ContractVersion)
	}
	if v := loadContract(t).Info.Version; v != ContractVersion {
		t.Errorf("pinned contract info.version = %q, ContractVersion = %q", v, ContractVersion)
	}
}

// sentRequest is what the fake server saw of one request.
type sentRequest struct {
	method    string
	path      string // relative to /api/v1
	mediaType string
	fields    map[string]bool // multipart field names or JSON body top-level keys
	query     map[string]bool
	header    http.Header
}

// contractCalls calls every public resource method once, with every optional
// argument that adds a field. Keys are "<ResourceType>.<Method>".
var contractCalls = map[string]func(context.Context, *Client) error{
	"FacesResource.Register": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Register(ctx, "col_1", RegisterInput{
			ExternalID: "emp-1", Image: []byte{0xff, 0xd8},
			Metadata: map[string]interface{}{"dept": "eng"}, LivenessToken: "lvt_1",
		}, WithIdempotencyKey("key-1"))
		return err
	},
	"FacesResource.List": func(ctx context.Context, c *Client) error {
		_, _, err := c.Faces.List(ctx, "col_1", ListOptions{Limit: 10, Offset: 5})
		return err
	},
	"FacesResource.Get": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Get(ctx, "col_1", "face_1")
		return err
	},
	"FacesResource.Delete": func(ctx context.Context, c *Client) error {
		return c.Faces.Delete(ctx, "col_1", "face_1")
	},
	"FacesResource.Verify": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Verify(ctx, "col_1", VerifyInput{Image: []byte{0xff, 0xd8}, FaceID: "face_1", Threshold: 0.5})
		return err
	},
	"FacesResource.Identify": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Identify(ctx, "col_1", IdentifyInput{Image: []byte{0xff, 0xd8}, TopK: 3, Threshold: 0.5})
		return err
	},
	"FacesResource.Liveness": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Liveness(ctx, "col_1", []byte{0xff, 0xd8}, "")
		return err
	},
	"FacesResource.ActiveLiveness": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.ActiveLiveness(ctx, "col_1", frames(5))
		return err
	},
	"FacesResource.CreateLivenessSession": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.CreateLivenessSession(ctx, "col_1")
		return err
	},
	"FacesResource.CompleteLivenessSession": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.CompleteLivenessSession(ctx, "col_1", "lvs_1", frames(5), true)
		return err
	},
	"FacesResource.Compare": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Compare(ctx, CompareInput{Image1: []byte{0xff, 0xd8}, Image2: []byte{0xff, 0xd8}, Threshold: 0.5})
		return err
	},
	"FacesResource.BatchRegister": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.BatchRegister(ctx, "col_1", contractBatchItems(), WithIdempotencyKey("key-2"))
		return err
	},
	"FacesResource.Attributes": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.Attributes(ctx, "col_1", []byte{0xff, 0xd8}, "")
		return err
	},
	"FacesResource.BatchRegisterAsync": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.BatchRegisterAsync(ctx, "col_1", contractBatchItems(), WithIdempotencyKey("key-3"))
		return err
	},
	"FacesResource.GetBatchJob": func(ctx context.Context, c *Client) error {
		_, err := c.Faces.GetBatchJob(ctx, "col_1", "job_1")
		return err
	},
}

func contractBatchItems() []BatchItem {
	return []BatchItem{
		{ExternalID: "emp-1", Image: []byte{0xff, 0xd8}, Metadata: map[string]interface{}{"dept": "eng"}, LivenessToken: "lvt_1"},
		{ExternalID: "emp-2", Image: []byte{0xff, 0xd8}},
	}
}

// TestContractCoversAllMethods fails when a public resource method is missing
// from contractCalls, so a new method cannot skip the contract check.
func TestContractCoversAllMethods(t *testing.T) {
	ct := reflect.TypeOf(Client{})
	for i := 0; i < ct.NumField(); i++ {
		f := ct.Field(i)
		if !f.IsExported() || f.Type.Kind() != reflect.Pointer || f.Type.Elem().Kind() != reflect.Struct {
			continue
		}
		for j := 0; j < f.Type.NumMethod(); j++ {
			key := f.Type.Elem().Name() + "." + f.Type.Method(j).Name
			if contractCalls[key] == nil {
				t.Errorf("%s is not called by the contract check; add it to contractCalls", key)
			}
		}
	}
}

func TestContractRequests(t *testing.T) {
	doc := loadContract(t)

	var mu sync.Mutex
	var sent []sentRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := sentRequest{
			method: r.Method,
			path:   strings.TrimPrefix(r.URL.Path, "/api/v1"),
			fields: map[string]bool{},
			query:  map[string]bool{},
			header: r.Header,
		}
		for k := range r.URL.Query() {
			req.query[k] = true
		}
		req.mediaType, _, _ = mime.ParseMediaType(r.Header.Get("Content-Type"))
		switch req.mediaType {
		case "multipart/form-data":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			} else {
				for k := range r.MultipartForm.File {
					req.fields[k] = true
				}
				for k := range r.MultipartForm.Value {
					req.fields[k] = true
				}
			}
		case "application/json":
			var body map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode JSON body: %v", err)
			}
			for k := range body {
				req.fields[k] = true
			}
		}
		mu.Lock()
		sent = append(sent, req)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	defer srv.Close()
	c := New("lxf_test", WithBaseURL(srv.URL+"/api/v1"))

	names := make([]string, 0, len(contractCalls))
	for name := range contractCalls {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		mu.Lock()
		sent = nil
		mu.Unlock()
		if err := contractCalls[name](context.Background(), c); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		mu.Lock()
		reqs := sent
		mu.Unlock()
		if len(reqs) == 0 {
			t.Errorf("%s: made no request", name)
		}
		for _, req := range reqs {
			if tmpl, err := checkContract(doc, req); err != "" {
				t.Errorf("%s: %s %s: %s", name, req.method, req.path, err)
			} else {
				t.Logf("%s -> %s %s", name, req.method, tmpl)
			}
		}
	}
}

var pathParam = regexp.MustCompile(`\{[^/]+\}`)

// checkContract returns the contract path template req matched, or a message
// saying how req breaks the contract.
func checkContract(doc *contractDoc, req sentRequest) (string, string) {
	// Literal segments beat parameters, as in the server's router, so try
	// templates with fewer parameters first.
	tmpls := make([]string, 0, len(doc.Paths))
	for tmpl := range doc.Paths {
		tmpls = append(tmpls, tmpl)
	}
	sort.Slice(tmpls, func(i, j int) bool {
		ni, nj := strings.Count(tmpls[i], "{"), strings.Count(tmpls[j], "{")
		return ni < nj || ni == nj && tmpls[i] < tmpls[j]
	})
	var pathFound bool
	for _, tmpl := range tmpls {
		// Each {param} matches exactly one path segment.
		parts := pathParam.Split(tmpl, -1)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		if !regexp.MustCompile("^" + strings.Join(parts, `[^/]+`) + "$").MatchString(req.path) {
			continue
		}
		pathFound = true
		op, ok := doc.Paths[tmpl][strings.ToLower(req.method)]
		if !ok {
			continue
		}
		return tmpl, checkRequired(doc, op, req)
	}
	if pathFound {
		return "", "the contract has this path but not this HTTP method"
	}
	return "", "no path in contract " + ContractVersion + " matches"
}

func checkRequired(doc *contractDoc, op contractOperation, req sentRequest) string {
	var missing []string
	for _, p := range op.Parameters {
		if !p.Required {
			continue
		}
		switch p.In {
		case "query":
			if !req.query[p.Name] {
				missing = append(missing, "query "+p.Name)
			}
		case "header":
			if req.header.Get(p.Name) == "" {
				missing = append(missing, "header "+p.Name)
			}
		}
	}
	if op.RequestBody != nil && len(op.RequestBody.Content) > 0 {
		body, ok := op.RequestBody.Content[req.mediaType]
		if !ok {
			return "sends a " + strings.TrimSpace(req.mediaType+" body") + "; the contract accepts none of that media type"
		}
		schema := body.Schema
		if schema.Ref != "" {
			schema = doc.Components.Schemas[strings.TrimPrefix(schema.Ref, "#/components/schemas/")]
		}
		for _, name := range schema.Required {
			if !sentField(req.fields, name) {
				missing = append(missing, "field "+name)
			}
		}
	}
	if len(missing) > 0 {
		return "does not send required " + strings.Join(missing, ", ")
	}
	return ""
}

var firstOfRepeated = regexp.MustCompile(`^(.*)(\[0\]|_0)$`)

// sentField reports whether name was sent. The contract documents a repeated
// file field by its first name only: images[0] stands for any images[N],
// frame_0 for any frame_N.
func sentField(fields map[string]bool, name string) bool {
	if fields[name] {
		return true
	}
	m := firstOfRepeated.FindStringSubmatch(name)
	if m == nil {
		return false
	}
	idx := `\[\d+\]`
	if m[2] == "_0" {
		idx = `_\d+`
	}
	re := regexp.MustCompile("^" + regexp.QuoteMeta(m[1]) + idx + "$")
	for f := range fields {
		if re.MatchString(f) {
			return true
		}
	}
	return false
}
