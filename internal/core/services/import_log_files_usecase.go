package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"svitorz/tracelog/internal/core/domain"
)

const maxImportedLogLineSize = 1024 * 1024

// ImportLogFilesInput defines the CLI input for file-based ingestion.
type ImportLogFilesInput struct {
	Files         []string
	ApplicationID string
	SourceID      string
	EnvironmentID string
}

// ImportLogFilesResult summarizes the quality of an import execution.
type ImportLogFilesResult struct {
	FilesProcessed   int
	LinesRead        int
	Persisted        int
	PersistenceError int
	// BlankSkipped counts empty or whitespace-only lines, which are not stored.
	BlankSkipped    int
	Normalized      int
	Partial         int
	Uncategorizable int
}

// ImportLogFilesUseCase ingests technical events from one or more files.
type ImportLogFilesUseCase struct {
	registerUC  *RegisterTechnicalEventUseCase
	openFile    func(string) (io.ReadCloser, error)
	now         func() time.Time
	strategies  []logLineParserStrategy
	categorizer logLineCategorizer
}

func NewImportLogFilesUseCase(registerUC *RegisterTechnicalEventUseCase) *ImportLogFilesUseCase {
	return &ImportLogFilesUseCase{
		registerUC: registerUC,
		openFile: func(path string) (io.ReadCloser, error) {
			return os.Open(path)
		},
		now: time.Now,
		strategies: []logLineParserStrategy{
			jsonLogLineParser{},
			timestampLevelLogLineParser{},
			plainTextLogLineParser{},
		},
		categorizer: defaultLogLineCategorizer{},
	}
}

func (uc *ImportLogFilesUseCase) Execute(ctx context.Context, input ImportLogFilesInput) (*ImportLogFilesResult, error) {
	if uc == nil || uc.registerUC == nil {
		return nil, fmt.Errorf("register technical event use case is required")
	}

	files := normalizeImportFileList(input.Files)
	if len(files) == 0 {
		return nil, fmt.Errorf("at least one --file must be provided")
	}

	result := &ImportLogFilesResult{}
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		file, err := uc.openFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open %q: %w", path, err)
		}

		if err := uc.importFile(ctx, file, path, input, result); err != nil {
			_ = file.Close()
			return nil, err
		}

		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("failed to close %q: %w", path, err)
		}

		result.FilesProcessed++
	}

	return result, nil
}

func (uc *ImportLogFilesUseCase) importFile(ctx context.Context, reader io.Reader, path string, input ImportLogFilesInput, result *ImportLogFilesResult) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxImportedLogLineSize)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		result.LinesRead++

		rawLine := scanner.Text()
		if strings.TrimSpace(rawLine) == "" {
			result.BlankSkipped++
			continue
		}

		parsed := uc.parseLine(rawLine)
		parsed.Payload = enrichImportedPayload(parsed.Payload, path, lineNumber, parsed.Format, parsed.Parser)
		category, status := uc.categorizer.Categorize(parsed)

		registered, err := uc.registerUC.Execute(ctx, RegisterTechnicalEventInput{
			OccurredAt:          parsed.OccurredAt,
			Message:             parsed.Message,
			Category:            category,
			Payload:             parsed.Payload,
			TraceID:             parsed.TraceID,
			CorrelationID:       parsed.CorrelationID,
			RawLine:             parsed.RawLine,
			NormalizationStatus: status,
			ParseError:          parsed.ParseError,
			IngestionChannel:    domain.IngestionFile,
			ApplicationID:       input.ApplicationID,
			SourceID:            input.SourceID,
			EnvironmentID:       input.EnvironmentID,
		})
		if err != nil {
			result.PersistenceError++
			continue
		}

		result.Persisted++
		switch registered.Event.NormalizationStatus {
		case domain.NormalizationStatusNormalized:
			result.Normalized++
		case domain.NormalizationStatusPartial:
			result.Partial++
		case domain.NormalizationStatusUncategorizable:
			result.Uncategorizable++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read %q: %w", path, err)
	}

	return nil
}

func normalizeImportFileList(files []string) []string {
	if len(files) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		normalized = append(normalized, file)
	}

	return normalized
}

type parsedLogLine struct {
	OccurredAt          string
	Message             string
	Payload             map[string]any
	TraceID             string
	CorrelationID       string
	RawLine             string
	ParseError          string
	Format              string
	Parser              string
	NormalizationStatus domain.NormalizationStatus
}

type logLineParserStrategy interface {
	Name() string
	Detect(rawLine string) bool
	Parse(rawLine string, now time.Time) parsedLogLine
}

type logLineCategorizer interface {
	Categorize(parsed parsedLogLine) (domain.EventCategory, domain.NormalizationStatus)
}

func (uc *ImportLogFilesUseCase) parseLine(rawLine string) parsedLogLine {
	now := uc.now().UTC()

	for _, strategy := range uc.strategies {
		if !strategy.Detect(rawLine) {
			continue
		}

		parsed := strategy.Parse(rawLine, now)
		if parsed.RawLine == "" {
			parsed.RawLine = rawLine
		}
		if parsed.Parser == "" {
			parsed.Parser = strategy.Name()
		}
		if parsed.Format == "" {
			parsed.Format = strategy.Name()
		}
		if parsed.Payload == nil {
			parsed.Payload = map[string]any{}
		}

		return parsed
	}

	return plainTextLogLineParser{}.Parse(rawLine, now)
}

type jsonLogLineParser struct{}

func (jsonLogLineParser) Name() string { return "json" }

func (jsonLogLineParser) Detect(rawLine string) bool {
	trimmed := strings.TrimSpace(rawLine)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func (jsonLogLineParser) Parse(rawLine string, now time.Time) parsedLogLine {
	trimmed := strings.TrimSpace(rawLine)
	parsed := parsedLogLine{
		RawLine:             rawLine,
		Format:              "json",
		Parser:              "json",
		NormalizationStatus: domain.NormalizationStatusPartial,
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		fallback := plainTextLogLineParser{}.Parse(rawLine, now)
		fallback.Format = "json"
		fallback.Parser = "json-fallback"
		fallback.ParseError = err.Error()
		fallback.NormalizationStatus = domain.NormalizationStatusPartial
		return fallback
	}

	payload := map[string]any{}
	switch value := decoded.(type) {
	case map[string]any:
		payload = value
	case []any:
		payload["value"] = value
	default:
		payload["value"] = value
	}

	parsed.Payload = payload
	parsed.Message = extractFirstString(payload, "message", "msg", "error", "event", "description")
	parsed.TraceID = extractFirstString(payload, "trace_id", "traceId")
	parsed.CorrelationID = extractFirstString(payload, "correlation_id", "correlationId", "request_id", "requestId")

	if category := extractFirstString(payload, "category", "event_category"); category != "" {
		payload["category_hint"] = strings.ToLower(strings.TrimSpace(category))
	}

	if occurredAt, ok := extractOccurredAtFromPayload(payload); ok {
		parsed.OccurredAt = occurredAt
	}

	if level := extractFirstString(payload, "level", "severity", "log_level"); level != "" {
		payload["level"] = strings.ToLower(strings.TrimSpace(level))
	}

	if strings.TrimSpace(parsed.Message) == "" {
		parsed.Message = strings.TrimSpace(rawLine)
	}

	if parsed.OccurredAt != "" && strings.TrimSpace(parsed.Message) != "" {
		parsed.NormalizationStatus = domain.NormalizationStatusNormalized
	}

	return parsed
}

type timestampLevelLogLineParser struct{}

var timestampLevelPattern = regexp.MustCompile(`^\s*(\[[^\]]+\]|\d{4}-\d{2}-\d{2}[T ][^ ]+)\s+([A-Za-z]+)\s+(.*)$`)

func (timestampLevelLogLineParser) Name() string { return "timestamp_level_message" }

func (timestampLevelLogLineParser) Detect(rawLine string) bool {
	return timestampLevelPattern.MatchString(rawLine)
}

func (timestampLevelLogLineParser) Parse(rawLine string, now time.Time) parsedLogLine {
	matches := timestampLevelPattern.FindStringSubmatch(rawLine)
	if len(matches) != 4 {
		return plainTextLogLineParser{}.Parse(rawLine, now)
	}

	timestampToken := strings.Trim(matches[1], "[]")
	level := strings.ToLower(strings.TrimSpace(matches[2]))
	message := strings.TrimSpace(matches[3])

	payload := map[string]any{
		"level": level,
	}
	if ts, ok := parseKnownTimestamp(timestampToken); ok {
		return parsedLogLine{
			OccurredAt:          ts.Format(time.RFC3339Nano),
			Message:             message,
			Payload:             payload,
			RawLine:             rawLine,
			Format:              "timestamp_level_message",
			Parser:              "timestamp_level_message",
			NormalizationStatus: domain.NormalizationStatusNormalized,
		}
	}

	return parsedLogLine{
		Message:             firstNonEmpty(message, strings.TrimSpace(rawLine)),
		Payload:             payload,
		RawLine:             rawLine,
		ParseError:          fmt.Sprintf("timestamp %q could not be parsed", timestampToken),
		Format:              "timestamp_level_message",
		Parser:              "timestamp_level_message",
		NormalizationStatus: domain.NormalizationStatusPartial,
	}
}

type plainTextLogLineParser struct{}

func (plainTextLogLineParser) Name() string { return "plain_text" }

func (plainTextLogLineParser) Detect(string) bool { return true }

func (plainTextLogLineParser) Parse(rawLine string, _ time.Time) parsedLogLine {
	message := strings.TrimSpace(rawLine)
	status := domain.NormalizationStatusPartial
	if message == "" {
		status = domain.NormalizationStatusUncategorizable
	}

	return parsedLogLine{
		Message:             message,
		Payload:             map[string]any{},
		RawLine:             rawLine,
		Format:              "plain_text",
		Parser:              "plain_text",
		NormalizationStatus: status,
	}
}

type defaultLogLineCategorizer struct{}

func (defaultLogLineCategorizer) Categorize(parsed parsedLogLine) (domain.EventCategory, domain.NormalizationStatus) {
	if value, ok := parsed.Payload["category_hint"]; ok {
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
		case string(domain.CategoryApplication):
			return domain.CategoryApplication, domain.NormalizationStatusNormalized
		case string(domain.CategorySystem):
			return domain.CategorySystem, domain.NormalizationStatusNormalized
		case string(domain.CategorySecurity):
			return domain.CategorySecurity, domain.NormalizationStatusNormalized
		case string(domain.CategoryAudit):
			return domain.CategoryAudit, domain.NormalizationStatusNormalized
		}
	}

	content := strings.ToLower(strings.TrimSpace(parsed.Message))
	words := messageWords(content)
	switch {
	// Security first: "login failed" and "unauthorized" also contain audit words (spec 0004).
	case hasAnyWord(words, securityWords...) ||
		(hasAnyWord(words, authenticationWords...) && hasAnyWord(words, authFailureWords...)):
		return domain.CategorySecurity, promoteNormalizationStatus(parsed.NormalizationStatus, domain.NormalizationStatusNormalized)
	case hasAnyWord(words, auditWords...) || containsAny(content, auditPhrases...):
		return domain.CategoryAudit, promoteNormalizationStatus(parsed.NormalizationStatus, domain.NormalizationStatusNormalized)
	case hasAnyWord(words, systemWords...):
		return domain.CategorySystem, promoteNormalizationStatus(parsed.NormalizationStatus, domain.NormalizationStatusNormalized)
	case parsed.Format != "plain_text" && content != "":
		return domain.CategoryApplication, promoteNormalizationStatus(parsed.NormalizationStatus, domain.NormalizationStatusPartial)
	default:
		return domain.CategoryUnknown, domain.NormalizationStatusUncategorizable
	}
}

// Category keywords, matched as whole words (spec 0004, seção 4.4).
var (
	securityWords       = []string{"unauthorized", "forbidden", "denied", "intrusion", "security", "credential", "credentials", "token", "tokens"}
	authenticationWords = []string{"login", "logon", "signin", "auth", "authentication"}
	authFailureWords    = []string{"failed", "failure", "invalid", "rejected", "locked"}
	auditWords          = []string{"login", "logout", "auth", "authentication", "permission"}
	auditPhrases        = []string{"access granted", "user action", "created by", "updated by"}
	systemWords         = []string{"database", "disk", "memory", "cpu", "network", "timeout", "timeouts", "connection", "connections", "postgres", "redis", "kafka"}
)

func promoteNormalizationStatus(current domain.NormalizationStatus, desired domain.NormalizationStatus) domain.NormalizationStatus {
	if current == domain.NormalizationStatusNormalized {
		return current
	}
	if current == "" {
		return desired
	}
	if desired == domain.NormalizationStatusNormalized {
		return desired
	}
	return current
}

func enrichImportedPayload(payload map[string]any, filePath string, lineNumber int, format string, parser string) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}

	ingestion := map[string]any{}
	if existing, ok := payload["_ingestion"].(map[string]any); ok {
		for key, value := range existing {
			ingestion[key] = value
		}
	}

	ingestion["file_path"] = filePath
	ingestion["line_number"] = lineNumber
	ingestion["detected_format"] = format
	ingestion["parser"] = parser
	payload["_ingestion"] = ingestion

	return payload
}

func extractOccurredAtFromPayload(payload map[string]any) (string, bool) {
	for _, key := range []string{"occurred_at", "timestamp", "time", "@timestamp", "ts"} {
		value, ok := payload[key]
		if !ok {
			continue
		}

		switch v := value.(type) {
		case string:
			if parsed, ok := parseKnownTimestamp(v); ok {
				return parsed.Format(time.RFC3339Nano), true
			}
		case float64:
			if parsed, ok := parseUnixTimestamp(int64(v)); ok {
				return parsed.Format(time.RFC3339Nano), true
			}
		case json.Number:
			if n, err := v.Int64(); err == nil {
				if parsed, ok := parseUnixTimestamp(n); ok {
					return parsed.Format(time.RFC3339Nano), true
				}
			}
		}
	}

	return "", false
}

func extractFirstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}

		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
			return text
		}
	}

	return ""
}

func parseKnownTimestamp(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05,000",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			return parsed.UTC(), true
		}
	}

	if unixSeconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return parseUnixTimestamp(unixSeconds)
	}

	return time.Time{}, false
}

func parseUnixTimestamp(value int64) (time.Time, bool) {
	switch {
	case value > 999999999999:
		return time.UnixMilli(value).UTC(), true
	case value > 0:
		return time.Unix(value, 0).UTC(), true
	default:
		return time.Time{}, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}
