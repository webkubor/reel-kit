package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

type testSecrets struct{ data map[string]string }

func (s *testSecrets) Get(service, account string) (string, error) {
	v, ok := s.data[service+account]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}
func (s *testSecrets) Set(service, account, value string) error {
	s.data[service+account] = value
	return nil
}
func (s *testSecrets) Delete(service, account string) error {
	delete(s.data, service+account)
	return nil
}

// Matrix: URL and secret separation, read-only probe, successful storyboard,
// unknown actual model, invalid input and cancellation.
// Failure matrix below: HTTP failure/no retry, malformed JSON, missing choices,
// invalid storyboard with model receipt retained, deadline, corrupt settings.
func TestAIStoryboard(t *testing.T) {
	fake := &testSecrets{data: map[string]string{}}
	s := newTestStore(t)
	project, err := s.AddProject("test")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{store: s, aiSecrets: fake}
	sent := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing credential")
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/v1/models" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		sent++
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["model"] != "model-a" {
			t.Error("invalid request")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"title":"一支笔","subtitle":"","ratio":"9:16","lines":["这是我的笔。"]}`}}}})
	}))
	t.Cleanup(server.Close)
	settings, err := e.SaveAISettings(AISettings{BaseURL: server.URL + "/v1", Model: "model-a"}, "test-secret")
	if err != nil || !settings.HasKey {
		t.Fatalf("save: %+v %v", settings, err)
	}
	config, err := os.ReadFile(filepath.Join(s.Root(), "ai-settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "test-secret") {
		t.Fatal("secret persisted in settings")
	}
	if _, err = e.CheckAIConnection(); err != nil || sent != 0 {
		t.Fatalf("probe made a generation or failed: %v", err)
	}
	job := &Job{ProjectID: project.ID, Input: map[string]any{"prompt": "介绍这支笔"}}
	id, err := e.generateStoryboard(context.Background(), job, func(string, map[string]any) {}, func(Step) {})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := e.ReadStoryboard(project.ID, id)
	if err != nil || len(plan.Lines) != 1 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	asset, err := s.Asset(id, project.ID)
	if err != nil || asset.Prompt != "介绍这支笔" || asset.Extra["actualModel"] != "unknown" || asset.Extra["requestedModel"] != "model-a" {
		t.Fatalf("metadata: %+v %v", asset, err)
	}
	if _, _, err = e.aiRequest(context.Background(), "POST", "/unused", map[string]any{"secret": make(chan int)}); err == nil {
		t.Fatal("expected JSON error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = e.aiRequest(ctx, "GET", "/models", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	changed, err := e.SaveAISettings(AISettings{BaseURL: "http://localhost:12345/v1", Model: "model-a"}, "")
	if err != nil || changed.HasKey {
		t.Fatal("key leaked across API addresses")
	}
}
func TestStoryboardFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		actual string
	}{
		{"HTTP error", 429, `private upstream diagnostic`, "unknown"},
		{"malformed response", 200, `{`, "unknown"},
		{"missing choices", 200, `{}`, "unknown"},
		{"invalid plan with receipt", 200, `{"model":"actual-model","choices":[{"message":{"content":"{}"}}]}`, "actual-model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			p, err := s.AddProject("test")
			if err != nil {
				t.Fatal(err)
			}
			e := &Engine{store: s, aiSecrets: &testSecrets{data: map[string]string{}}}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			if _, err = e.SaveAISettings(AISettings{BaseURL: server.URL, Model: "requested-model"}, "test-secret"); err != nil {
				t.Fatal(err)
			}
			var receipt map[string]any
			id, err := e.generateStoryboard(context.Background(), &Job{ProjectID: p.ID, Input: map[string]any{"prompt": "介绍产品"}}, func(_ string, value map[string]any) {
				if value != nil {
					receipt = value
				}
			}, func(Step) {})
			if err == nil || id != "" || len(s.State().Assets) != 0 {
				t.Fatalf("invalid response produced asset: %s %v", id, err)
			}
			if calls != 1 {
				t.Fatalf("expected one attempt, got %d", calls)
			}
			if strings.Contains(err.Error(), "private upstream diagnostic") {
				t.Fatal("upstream body exposed")
			}
			if receipt["requestedModel"] != "requested-model" || receipt["actualModel"] != tc.actual {
				t.Fatalf("receipt lost: %+v", receipt)
			}
		})
	}
}

func TestAISettingsFailurePaths(t *testing.T) {
	s := newTestStore(t)
	e := &Engine{store: s, aiSecrets: &testSecrets{data: map[string]string{}}}
	if _, _, err := e.aiRequest(context.Background(), "GET", "/models", nil); err == nil {
		t.Fatal("unconfigured request allowed")
	}
	if _, err := e.SaveAISettings(AISettings{BaseURL: "http://localhost:12345/v1", Model: "m"}, "test-secret"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	t.Cleanup(cancel)
	if _, _, err := e.aiRequest(ctx, "GET", "/models", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline: %v", err)
	}
	if err := e.ClearAIKey(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.aiRequest(context.Background(), "GET", "/models", nil); err == nil {
		t.Fatal("request without key allowed")
	}
	path := filepath.Join(s.Root(), "ai-settings.json")
	if err := os.WriteFile(path, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.GetAISettings(); err == nil {
		t.Fatal("corrupt settings ignored")
	}
	if _, err := e.SaveAISettings(AISettings{BaseURL: "http://localhost:12345/v1", Model: "m"}, ""); err == nil {
		t.Fatal("corrupt settings overwritten")
	}
}

func TestValidateAIAndStoryboard(t *testing.T) {
	for _, raw := range []string{"http://example.com/v1", "https://user:secret@example.com/v1", "https://example.com/v1?key=x"} {
		if _, err := validateAISettings(AISettings{BaseURL: raw, Model: "m"}); err == nil {
			t.Errorf("accepted unsafe URL %s", raw)
		}
	}
	for _, text := range []string{`{}`, `{"ratio":"1:1","lines":["x"]}`, `{"ratio":"9:16","lines":[]}`, `{"ratio":"9:16","lines":[""]}`} {
		if _, err := parseStoryboard(text); err == nil {
			t.Errorf("accepted invalid plan %s", text)
		}
	}
	if _, err := parseStoryboard("```json\n{\"ratio\":\"9:16\",\"lines\":[\"一\"]}\n```"); err != nil {
		t.Fatal(err)
	}
}
