package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
	"svitorz/tracelog/internal/core/services"
)

// As respostas da API de leitura são comparadas com testdata/api/*.json, que também
// servem de mocks para o front (spec 0005, seção 5.4). Para regravar depois de uma
// mudança consciente de contrato: go test ./internal/adapters/http/ -run ReadAPIFixtures -update
var updateFixtures = flag.Bool("update", false, "rewrite testdata/api fixtures")

const (
	fixtureEventFatal = "0f3c2a9e-1b7d-4c55-9a7e-2d8f1e6b4a10"
	fixtureEventError = "1a7b3c5d-2e4f-4a6b-8c9d-0e1f2a3b4c5d"
	fixtureEventXSS   = "2b8c4d6e-3f5a-4b7c-9d0e-1f2a3b4c5d6e"
	fixtureMissingID  = "ffffffff-ffff-4fff-bfff-ffffffffffff"
)

func fixtureTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return t
}

func fixtureEvents() []domain.TechnicalEvent {
	return []domain.TechnicalEvent{
		{
			ID:                  fixtureEventFatal,
			ExternalID:          "file:3b6f0c1e9a7d4b2f8c5e1a0d9b7c6e4f",
			OccurredAt:          fixtureTime("2026-03-10T14:47:00Z"),
			ReceivedAt:          fixtureTime("2026-11-23T20:38:40.512345Z"),
			CreatedAt:           fixtureTime("2026-11-23T20:38:40.514001Z"),
			Message:             "order worker crashed: connection pool exhausted",
			Severity:            domain.SeverityFatal,
			Category:            domain.CategorySystem,
			Payload:             map[string]any{"level": "fatal", "trace_id": "9c1d7e0a4b3f2e18"},
			TraceID:             "9c1d7e0a4b3f2e18",
			RawLine:             `{"level":"fatal","message":"order worker crashed: connection pool exhausted","timestamp":"2026-03-10T14:47:00Z","trace_id":"9c1d7e0a4b3f2e18"}`,
			NormalizationStatus: domain.NormalizationStatusNormalized,
			IngestionChannel:    domain.IngestionFile,
			ApplicationID:       testAppID,
			SourceID:            testSrcID,
			EnvironmentID:       testEnvID,
		},
		{
			// Mesmo occurred_at do fatal: o id desempata (id DESC põe este primeiro).
			ID:                  fixtureEventError,
			OccurredAt:          fixtureTime("2026-03-10T14:47:00Z"),
			ReceivedAt:          fixtureTime("2026-11-23T20:38:40.498765Z"),
			CreatedAt:           fixtureTime("2026-11-23T20:38:40.500002Z"),
			Message:             "payment provider timeout",
			Severity:            domain.SeverityError,
			Category:            domain.CategorySystem,
			Payload:             map[string]any{"provider": "acme-pay"},
			CorrelationID:       "req-42",
			NormalizationStatus: domain.NormalizationStatusNormalized,
			IngestionChannel:    domain.IngestionHTTP,
			ApplicationID:       testAppID,
			SourceID:            testSrcID,
			EnvironmentID:       testEnvID,
		},
		{
			// Corpo HTTP que não era JSON: mensagem padrão, corpo bruto em _ingestion.
			ID:         fixtureEventXSS,
			OccurredAt: fixtureTime("2026-03-09T08:00:00Z"),
			ReceivedAt: fixtureTime("2026-03-09T08:00:00.250000Z"),
			CreatedAt:  fixtureTime("2026-03-09T08:00:00.251000Z"),
			Message:    `<script>alert("xss")</script>`,
			Severity:   domain.SeverityInfo,
			Category:   domain.CategoryUnknown,
			Payload: map[string]any{"_ingestion": map[string]any{
				"decode_error":           "invalid character '<' looking for beginning of value",
				"raw_request_body":       `<img src=x onerror=alert(1)>`,
				"normalization_warnings": []any{"occurred_at missing or invalid; received_at used"},
			}},
			ParseError:          "invalid character '<' looking for beginning of value",
			NormalizationStatus: domain.NormalizationStatusPartial,
			IngestionChannel:    domain.IngestionHTTP,
			ApplicationID:       testAppID,
			SourceID:            testSrcID,
			EnvironmentID:       testEnvID,
		},
	}
}

// readEventRepository orders and pages like the PostgreSQL adapter.
type readEventRepository struct {
	events []domain.TechnicalEvent
	err    error
}

func (r *readEventRepository) Save(context.Context, *domain.TechnicalEvent) (bool, error) {
	return false, errors.New("not used")
}

func (r *readEventRepository) FindByFilters(_ context.Context, f ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	if r.err != nil {
		return nil, r.err
	}
	sorted := append([]domain.TechnicalEvent(nil), r.events...)
	newer := func(a domain.TechnicalEvent, t time.Time, id string) bool {
		if !a.OccurredAt.Equal(t) {
			return a.OccurredAt.After(t)
		}
		return a.ID > id
	}
	sort.Slice(sorted, func(i, j int) bool { return newer(sorted[i], sorted[j].OccurredAt, sorted[j].ID) })

	out := []domain.TechnicalEvent{}
	for _, e := range sorted {
		if f.Severity != "" && e.Severity != f.Severity {
			continue
		}
		// Keyset: keep only events strictly older than the cursor.
		if f.After != nil && !newer(domain.TechnicalEvent{OccurredAt: f.After.OccurredAt, ID: f.After.ID}, e.OccurredAt, e.ID) {
			continue
		}
		out = append(out, e)
		if len(out) == f.Limit {
			break
		}
	}
	return out, nil
}

func (r *readEventRepository) FindByID(_ context.Context, id string) (domain.TechnicalEvent, error) {
	if r.err != nil {
		return domain.TechnicalEvent{}, r.err
	}
	for _, e := range r.events {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.TechnicalEvent{}, ports.ErrNotFound
}

type readFrequencyRepository struct{}

func (readFrequencyRepository) GetErrorFrequencyByDay(context.Context, ports.ErrorFrequencyFilters) ([]domain.ErrorFrequency, error) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	return []domain.ErrorFrequency{{Date: day(1), Errors: 2}, {Date: day(2), Errors: 2}, {Date: day(3), Errors: 1}}, nil
}

type readStatsRepository struct {
	dataRange ports.EventDataRange
}

func (readStatsRepository) CountBySeverity(context.Context, ports.EventStatsFilters) (map[domain.Severity]int64, error) {
	return map[domain.Severity]int64{domain.SeverityInfo: 266, domain.SeverityWarn: 73, domain.SeverityError: 38, domain.SeverityFatal: 1}, nil
}

func (r readStatsRepository) DataRange(context.Context) (ports.EventDataRange, error) {
	return r.dataRange, nil
}

type readReferenceRepository struct{}

func (readReferenceRepository) ListApplications(context.Context) ([]domain.MonitoredApplication, error) {
	return []domain.MonitoredApplication{{ID: testAppID, Name: "Local Application", ServiceKey: "local-app", CreatedAt: fixtureTime("2026-11-23T20:38:39.101112Z")}}, nil
}

func (readReferenceRepository) ListEnvironments(context.Context) ([]domain.Environment, error) {
	return []domain.Environment{{ID: testEnvID, Code: "local", Name: "Local", Description: "Local development environment", CreatedAt: fixtureTime("2026-11-23T20:38:39.098001Z")}}, nil
}

func (readReferenceRepository) ListSources(context.Context) ([]domain.EventSource, error) {
	return []domain.EventSource{{ID: testSrcID, Name: "HTTP API", SourceType: "http", CreatedAt: fixtureTime("2026-11-23T20:38:39.103004Z")}}, nil
}

type readTestOptions struct {
	eventErr  error
	dataRange ports.EventDataRange
	logs      *bytes.Buffer
}

func newReadTestServer(t *testing.T, opts readTestOptions) http.Handler {
	t.Helper()
	events := &readEventRepository{events: fixtureEvents(), err: opts.eventErr}
	stats := readStatsRepository{dataRange: opts.dataRange}
	clock := func() time.Time { return fixtureTime("2026-11-23T20:00:00Z") }
	logs := opts.logs
	if logs == nil {
		logs = &bytes.Buffer{}
	}

	read := &ReadServices{
		ListEvents:     services.NewListEventsPageUseCase(events),
		GetEvent:       services.NewGetTechnicalEventUseCase(events),
		ErrorFrequency: services.NewGetDailyErrorFrequencyUseCase(readFrequencyRepository{}),
		SeverityCounts: services.NewGetSeverityCountsUseCase(stats),
		DataRange:      services.NewGetDataRangeUseCase(stats, clock),
		Reference:      services.NewListReferenceDataUseCase(readReferenceRepository{}),
	}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	return NewServer(ServerConfig{MaxBodyBytes: 1024, Logger: logger, Read: read}, nil).httpServer.Handler
}

func doRequest(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func assertFixture(t *testing.T, name string, body []byte) {
	t.Helper()
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, body)
	}
	pretty.WriteByte('\n')

	path := filepath.Join("testdata", "api", name)
	if *updateFixtures {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture (run with -update to create it): %v", err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("response differs from %s\n--- got ---\n%s--- want ---\n%s", path, pretty.Bytes(), want)
	}
}

func TestReadAPIFixtures(t *testing.T) {
	full := newReadTestServer(t, readTestOptions{dataRange: ports.EventDataRange{
		FirstOccurredAt: ptrTime(fixtureTime("2026-03-01T08:46:09Z")),
		LastOccurredAt:  ptrTime(fixtureTime("2026-11-23T20:38:40.620158Z")),
		LastErrorAt:     ptrTime(fixtureTime("2026-03-14T21:12:00Z")),
	}})
	empty := newReadTestServer(t, readTestOptions{})
	failing := newReadTestServer(t, readTestOptions{eventErr: errors.New("db down")})

	firstPage := doRequest(full, http.MethodGet, "/api/v1/events?limit=2")
	var page EventListResponse
	if err := json.Unmarshal(firstPage.Body.Bytes(), &page); err != nil || page.NextCursor == nil {
		t.Fatalf("first page: %v, next_cursor = %v", err, page.NextCursor)
	}

	tests := []struct {
		fixture string
		h       http.Handler
		method  string
		target  string
		status  int
	}{
		{"events_page.json", full, http.MethodGet, "/api/v1/events?limit=2", 200},
		{"events_last_page.json", full, http.MethodGet, "/api/v1/events?limit=2&cursor=" + *page.NextCursor, 200},
		{"event_detail.json", full, http.MethodGet, "/api/v1/events/" + fixtureEventXSS, 200},
		{"error_frequency.json", full, http.MethodGet, "/api/v1/metrics/error-frequency?from=2026-03-01&to=2026-03-04", 200},
		{"severity_counts.json", full, http.MethodGet, "/api/v1/metrics/severity-counts?from=2026-03-01&to=2026-03-14", 200},
		{"data_range.json", full, http.MethodGet, "/api/v1/metrics/data-range", 200},
		{"data_range_empty.json", empty, http.MethodGet, "/api/v1/metrics/data-range", 200},
		{"applications.json", full, http.MethodGet, "/api/v1/applications", 200},
		{"environments.json", full, http.MethodGet, "/api/v1/environments", 200},
		{"sources.json", full, http.MethodGet, "/api/v1/sources", 200},
		{"error_invalid_parameter.json", full, http.MethodGet, "/api/v1/events?limit=201", 400},
		{"error_invalid_cursor.json", full, http.MethodGet, "/api/v1/events?cursor=not-a-cursor", 400},
		{"error_not_found.json", full, http.MethodGet, "/api/v1/events/" + fixtureMissingID, 404},
		{"error_method_not_allowed.json", full, http.MethodPost, "/api/v1/events", 405},
		{"error_internal.json", failing, http.MethodGet, "/api/v1/events", 500},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			rec := doRequest(tt.h, tt.method, tt.target)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.status, rec.Body)
			}
			assertFixture(t, tt.fixture, rec.Body.Bytes())
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestReadAPIRejectsInvalidParameters(t *testing.T) {
	h := newReadTestServer(t, readTestOptions{})

	tests := []struct {
		target string
		field  string
	}{
		{"/api/v1/events?limit=0", "limit"},
		{"/api/v1/events?limit=201", "limit"},
		{"/api/v1/events?limit=abc", "limit"},
		{"/api/v1/events?limit=1.5", "limit"},
		{"/api/v1/events?severity=critical", "severity"},
		{"/api/v1/events?application_id=xyz", "application_id"},
		{"/api/v1/events?environment_id=xyz", "environment_id"},
		{"/api/v1/events?from=2026-13-01", "from"},
		{"/api/v1/events?to=yesterday", "to"},
		{"/api/v1/events?from=2026-03-02&to=2026-03-01", "from"},
		{"/api/v1/events?severty=error", "severty"},
		{"/api/v1/events?severity=error&severity=fatal", "severity"},
		{"/api/v1/events/abc", "id"},
		{"/api/v1/events/" + fixtureEventFatal + "?x=1", "x"},
		{"/api/v1/metrics/error-frequency?to=2026-03-01", "from"},
		{"/api/v1/metrics/error-frequency?from=2026-03-01", "to"},
		{"/api/v1/metrics/error-frequency?from=2025-01-01&to=2026-03-01", "to"},
		{"/api/v1/metrics/error-frequency?from=2026-03-01&to=2026-03-02&severity=error", "severity"},
		{"/api/v1/metrics/severity-counts?application_id=1", "application_id"},
		{"/api/v1/metrics/data-range?from=2026-03-01", "from"},
		{"/api/v1/applications?limit=1", "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			rec := doRequest(h, http.MethodGet, tt.target)
			var body ReadErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %s", rec.Body)
			}
			if rec.Code != http.StatusBadRequest || body.Code != codeInvalidParameter || body.Field != tt.field || body.Error == "" {
				t.Fatalf("got %d %+v, want 400 invalid_parameter field %q", rec.Code, body, tt.field)
			}
		})
	}
}

func TestReadAPIAcceptsEmptyValuesAsAbsent(t *testing.T) {
	h := newReadTestServer(t, readTestOptions{})
	rec := doRequest(h, http.MethodGet, "/api/v1/events?severity=&limit=&cursor=")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
}

func TestReadAPIRejectsTamperedCursor(t *testing.T) {
	h := newReadTestServer(t, readTestOptions{})
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	cursors := map[string]string{
		"not base64":   "%%%",
		"padded":       enc(`{"t":"2026-03-10T14:47:00Z","id":"`+fixtureEventFatal+`"}`) + "==",
		"not JSON":     enc("hello"),
		"bad time":     enc(`{"t":"yesterday","id":"` + fixtureEventFatal + `"}`),
		"bad id":       enc(`{"t":"2026-03-10T14:47:00Z","id":"1 OR 1=1"}`),
		"extra field":  enc(`{"t":"2026-03-10T14:47:00Z","id":"` + fixtureEventFatal + `","x":1}`),
		"missing time": enc(`{"id":"` + fixtureEventFatal + `"}`),
	}
	for name, cursor := range cursors {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(h, http.MethodGet, "/api/v1/events?cursor="+url.QueryEscape(cursor))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_cursor"`) {
				t.Fatalf("got %d %s, want 400 invalid_cursor", rec.Code, rec.Body)
			}
		})
	}
}

func TestReadAPIEscapesHTMLAndSetsHeaders(t *testing.T) {
	h := newReadTestServer(t, readTestOptions{})
	rec := doRequest(h, http.MethodGet, "/api/v1/events/"+fixtureEventXSS)

	body := rec.Body.String()
	if strings.Contains(body, "<script>") || strings.Contains(body, "<img") || !strings.Contains(body, `\u003cscript\u003e`) {
		t.Fatalf("HTML must be escaped in the JSON body: %s", body)
	}
	for header, want := range map[string]string{
		"Content-Type":           "application/json",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	var detail EventDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || detail.Message != `<script>alert("xss")</script>` {
		t.Fatalf("decoded message = %q, %v; want the original text", detail.Message, err)
	}
}

func TestReadAPIHidesInternalErrors(t *testing.T) {
	logs := &bytes.Buffer{}
	h := newReadTestServer(t, readTestOptions{eventErr: errors.New(`pq: relation "secret_table" does not exist`), logs: logs})

	for _, target := range []string{"/api/v1/events", "/api/v1/events/" + fixtureEventFatal} {
		rec := doRequest(h, http.MethodGet, target)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret_table") {
			t.Fatalf("%s: got %d %s, want 500 without internal details", target, rec.Code, rec.Body)
		}
	}
	if !strings.Contains(logs.String(), "secret_table") {
		t.Fatalf("internal error must be logged; logs: %s", logs)
	}
}

func TestReadAPIRoutingEdges(t *testing.T) {
	h := newReadTestServer(t, readTestOptions{})

	tests := []struct {
		method, target string
		status         int
		code           string
	}{
		{http.MethodGet, "/api/v1/nada", 404, codeNotFound},
		{http.MethodGet, "/api/v1/events/", 404, codeNotFound},
		{http.MethodGet, "/api/v1/events/" + fixtureEventFatal + "/extra", 404, codeNotFound},
		{http.MethodDelete, "/api/v1/metrics/data-range", 405, codeMethodNotAllowed},
		{http.MethodPut, "/api/v1/applications", 405, codeMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			rec := doRequest(h, tt.method, tt.target)
			if rec.Code != tt.status || !strings.Contains(rec.Body.String(), `"code":"`+tt.code+`"`) {
				t.Fatalf("got %d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
			if rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("headers = %v", rec.Header())
			}
			if tt.status == 405 && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
			}
		})
	}

	// A ingestão não muda: GET /events continua 405 com o formato antigo.
	rec := doRequest(h, http.MethodGet, "/events")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" || strings.Contains(rec.Body.String(), "code") {
		t.Fatalf("GET /events = %d %s, want the unchanged 405", rec.Code, rec.Body)
	}

	head := doRequest(h, http.MethodHead, "/api/v1/applications")
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD = %d, want 200", head.Code)
	}
}

func TestReadAPIFiltersBySeverity(t *testing.T) {
	h := newReadTestServer(t, readTestOptions{})
	rec := doRequest(h, http.MethodGet, "/api/v1/events?severity=FATAL")

	var page EventListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(page.Items) != 1 || page.Items[0].ID != fixtureEventFatal || page.NextCursor != nil {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}
