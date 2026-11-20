package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
	"svitorz/tracelog/internal/core/services"
)

const (
	testAppID = "550e8400-e29b-41d4-a716-446655440010"
	testEnvID = "770e8400-e29b-41d4-a716-446655440012"
	testSrcID = "660e8400-e29b-41d4-a716-446655440011"
)

type ctxKey string

// fakeEventRepository keeps events in memory.
type fakeEventRepository struct {
	mu      sync.Mutex
	saved   []domain.TechnicalEvent
	lastCtx context.Context
	saveErr error
}

func (r *fakeEventRepository) Save(ctx context.Context, event *domain.TechnicalEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastCtx = ctx
	if r.saveErr != nil {
		return r.saveErr
	}

	event.ID = fmt.Sprintf("event-%d", len(r.saved)+1)
	r.saved = append(r.saved, *event)
	return nil
}

func (r *fakeEventRepository) FindByFilters(context.Context, ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	return nil, nil
}

func (r *fakeEventRepository) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.saved)
}

type fakeSetupRepository struct{}

func (fakeSetupRepository) SaveBasicSetup(_ context.Context, setup ports.BasicSystemSetup) (*ports.BasicSystemSetup, error) {
	return &setup, nil
}

func (fakeSetupRepository) HasBasicSetup(context.Context) (bool, error) { return true, nil }

func (fakeSetupRepository) LoadBasicSetup(context.Context) (*ports.BasicSystemSetup, error) {
	return &ports.BasicSystemSetup{
		Application: domain.MonitoredApplication{ID: testAppID},
		Environment: domain.Environment{ID: testEnvID},
		Source:      domain.EventSource{ID: testSrcID},
	}, nil
}

func newTestServer(t *testing.T, repo *fakeEventRepository, maxBodyBytes int64) http.Handler {
	t.Helper()
	uc := services.NewRegisterTechnicalEventUseCase(repo, fakeSetupRepository{})
	return NewServer(ServerConfig{MaxBodyBytes: maxBodyBytes}, uc).httpServer.Handler
}

func postEvent(t *testing.T, h http.Handler, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v (%q)", err, rec.Body.String())
	}
	return out
}

func TestRegisterEventCreatesEvent(t *testing.T) {
	repo := &fakeEventRepository{}
	h := newTestServer(t, repo, 1<<20)

	rec := postEvent(t, h, `{"external_id":"e-1","message":"user logged in","category":"audit"}`,
		map[string]string{"Content-Type": "application/json"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["external_id"]; got != "e-1" {
		t.Fatalf("external_id = %v, want e-1", got)
	}
	if repo.count() != 1 {
		t.Fatalf("saved events = %d, want 1", repo.count())
	}
}

func TestRegisterEventPersistsMalformedJSON(t *testing.T) {
	repo := &fakeEventRepository{}
	h := newTestServer(t, repo, 1<<20)

	rec := postEvent(t, h, `{not json`, nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: malformed JSON is still stored (ingestão tolerante)", rec.Code)
	}
	if repo.count() != 1 {
		t.Fatalf("saved events = %d, want 1", repo.count())
	}
	meta, _ := repo.saved[0].Payload["_ingestion"].(map[string]any)
	if meta["decode_error"] == nil || meta["raw_request_body"] != "{not json" {
		t.Fatalf("_ingestion = %#v, want decode_error and raw_request_body", meta)
	}
}

func TestRegisterEventRejectsWrongMethod(t *testing.T) {
	h := newTestServer(t, &fakeEventRepository{}, 1<<20)

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestRegisterEventRejectsBodyOverLimit(t *testing.T) {
	repo := &fakeEventRepository{}
	h := newTestServer(t, repo, 64)

	body := `{"message":"` + strings.Repeat("a", 100) + `"}`
	rec := postEvent(t, h, body, map[string]string{"Content-Type": "application/json"})

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if repo.count() != 0 {
		t.Fatalf("saved events = %d, want 0", repo.count())
	}
}

func TestRegisterEventPropagatesRequestContext(t *testing.T) {
	repo := &fakeEventRepository{}
	h := newTestServer(t, repo, 1<<20)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"message":"x"}`))
	req = req.WithContext(context.WithValue(req.Context(), ctxKey("probe"), "from-request"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if repo.lastCtx == nil || repo.lastCtx.Value(ctxKey("probe")) != "from-request" {
		t.Fatal("repository did not receive a context derived from the request")
	}
	if _, ok := repo.lastCtx.Deadline(); !ok {
		t.Fatal("repository context has no deadline, want the per-request timeout")
	}
}

func TestRegisterEventChecksContentType(t *testing.T) {
	tests := []struct {
		contentType string
		wantStatus  int
	}{
		{"", http.StatusCreated},
		{"application/json", http.StatusCreated},
		{"application/json; charset=utf-8", http.StatusCreated},
		{"application/vnd.api+json", http.StatusCreated},
		{"text/plain", http.StatusUnsupportedMediaType},
		{"application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"not a media type;;", http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			repo := &fakeEventRepository{}
			h := newTestServer(t, repo, 1<<20)

			header := map[string]string{}
			if tt.contentType != "" {
				header["Content-Type"] = tt.contentType
			}
			rec := postEvent(t, h, `{"message":"x"}`, header)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			wantSaved := 0
			if tt.wantStatus == http.StatusCreated {
				wantSaved = 1
			}
			if repo.count() != wantSaved {
				t.Fatalf("saved events = %d, want %d", repo.count(), wantSaved)
			}
		})
	}
}
