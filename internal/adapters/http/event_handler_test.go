package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	// knownApps, when non-nil, lists the application IDs that exist.
	knownApps map[string]bool
}

// Save mimics the PostgreSQL adapter: FK check on application_id against knownApps
// (when set) and deduplication by (application_id, external_id).
func (r *fakeEventRepository) Save(ctx context.Context, event *domain.TechnicalEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastCtx = ctx
	if r.saveErr != nil {
		return false, r.saveErr
	}
	if r.knownApps != nil && !r.knownApps[event.ApplicationID] {
		return false, fmt.Errorf("%w: application_id", ports.ErrReferenceNotFound)
	}

	if event.ExternalID != "" {
		for _, stored := range r.saved {
			if stored.ApplicationID == event.ApplicationID && stored.ExternalID == event.ExternalID {
				*event = stored
				return false, nil
			}
		}
	}

	event.ID = fmt.Sprintf("event-%d", len(r.saved)+1)
	r.saved = append(r.saved, *event)
	return true, nil
}

func (r *fakeEventRepository) FindByFilters(context.Context, ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	return nil, nil
}

func (r *fakeEventRepository) FindByID(context.Context, string) (domain.TechnicalEvent, error) {
	return domain.TechnicalEvent{}, ports.ErrNotFound
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
	return newTestServerWithLog(t, repo, maxBodyBytes, &bytes.Buffer{})
}

func newTestServerWithLog(t *testing.T, repo *fakeEventRepository, maxBodyBytes int64, logs *bytes.Buffer) http.Handler {
	t.Helper()
	uc := services.NewRegisterTechnicalEventUseCase(repo, fakeSetupRepository{})
	logger := slog.New(slog.NewTextHandler(logs, nil))
	return NewServer(ServerConfig{MaxBodyBytes: maxBodyBytes, Logger: logger}, uc).httpServer.Handler
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
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow = %q, want %q", got, http.MethodPost)
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

func TestRegisterEventHidesInternalErrors(t *testing.T) {
	internal := `ERROR: relation "technical_event" does not exist (SQLSTATE 42P01)`
	repo := &fakeEventRepository{saveErr: errors.New(internal)}
	logs := &bytes.Buffer{}
	h := newTestServerWithLog(t, repo, 1<<20, logs)

	rec := postEvent(t, h, `{"external_id":"e-1","message":"x"}`, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := decodeBody(t, rec)["error"]; got != "internal error" {
		t.Fatalf("error = %q, want generic message", got)
	}
	if strings.Contains(rec.Body.String(), "SQLSTATE") {
		t.Fatalf("response leaks internal details: %q", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "SQLSTATE 42P01") {
		t.Fatalf("server log = %q, want the internal error detail", logs.String())
	}
}

func TestRegisterEventIsIdempotentPerApplication(t *testing.T) {
	const otherAppID = "550e8400-e29b-41d4-a716-446655440099"
	repo := &fakeEventRepository{}
	h := newTestServer(t, repo, 1<<20)

	body := func(app string) string {
		return `{"external_id":"order-42","message":"x","application_id":"` + app + `"}`
	}

	first := postEvent(t, h, body(testAppID), nil)
	second := postEvent(t, h, body(testAppID), nil)
	otherApp := postEvent(t, h, body(otherAppID), nil)

	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", first.Code)
	}
	if second.Code != http.StatusOK {
		t.Fatalf("resend status = %d, want 200", second.Code)
	}
	if otherApp.Code != http.StatusCreated {
		t.Fatalf("other application status = %d, want 201", otherApp.Code)
	}

	firstBody, secondBody := decodeBody(t, first), decodeBody(t, second)
	if secondBody["duplicate"] != true || firstBody["duplicate"] != false {
		t.Fatalf("duplicate flags = %v then %v, want false then true", firstBody["duplicate"], secondBody["duplicate"])
	}
	if secondBody["id"] != firstBody["id"] {
		t.Fatalf("resend id = %v, want the original %v", secondBody["id"], firstBody["id"])
	}
	if repo.count() != 2 {
		t.Fatalf("stored events = %d, want 2 (one per application)", repo.count())
	}
}

func TestRegisterEventFallsBackWhenApplicationDoesNotExist(t *testing.T) {
	repo := &fakeEventRepository{knownApps: map[string]bool{testAppID: true}}
	h := newTestServer(t, repo, 1<<20)

	rec := postEvent(t, h, `{"message":"x","application_id":"11111111-1111-4111-8111-111111111111"}`, nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 after fallback (body %q)", rec.Code, rec.Body.String())
	}
	if got := repo.saved[0].ApplicationID; got != testAppID {
		t.Fatalf("stored application_id = %q, want fallback %q", got, testAppID)
	}
}
