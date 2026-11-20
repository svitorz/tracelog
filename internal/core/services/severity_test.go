package services

import (
	"testing"

	"svitorz/tracelog/internal/core/domain"
)

// Documenta a heurística de severidade por mensagem (spec 0002, seção 4.4). Casos marcados
// como "limitação" são falsos positivos conhecidos que a heurística NÃO resolve: ela não
// entende negação nem contexto, só palavras inteiras.
func TestDeriveSeverityFromMessage(t *testing.T) {
	tests := []struct {
		message string
		want    domain.Severity
		note    string
	}{
		// Palavras-chave como palavra inteira, com as flexões listadas.
		{"Database timeout", domain.SeverityError, ""},
		{"3 timeouts in a row", domain.SeverityError, ""},
		{"payment failed", domain.SeverityError, ""},
		{"ERROR: disk full", domain.SeverityError, ""},
		{"access denied for user", domain.SeverityError, ""},
		{"unhandled exceptions in worker", domain.SeverityError, ""},
		{"process crashed", domain.SeverityFatal, ""},
		{"goroutine panicked", domain.SeverityFatal, ""},
		{"unrecoverable state", domain.SeverityFatal, ""},
		{"retrying request", domain.SeverityWarn, ""},
		{"API v1 is deprecated", domain.SeverityWarn, ""},
		{"slow query detected", domain.SeverityWarn, ""},
		{"user profile loaded", domain.SeverityInfo, ""},
		{"deadline exceeded", domain.SeverityInfo, "sem palavra-chave conhecida"},

		// Nomes de tipo em CamelCase terminados em Error/Exception.
		{"caught NullPointerException in handler", domain.SeverityError, ""},
		{"TypeError: x is undefined", domain.SeverityError, ""},

		// Falsos positivos da busca por substring, removidos pela palavra inteira.
		{"terror alert dismissed", domain.SeverityInfo, "antes: error"},
		{"errorless run", domain.SeverityInfo, "antes: error"},
		{"crashpad uploaded report", domain.SeverityInfo, "antes: fatal"},
		{"slowapi middleware loaded", domain.SeverityInfo, "antes: warn"},

		// Limitações conhecidas: continuam classificados como erro.
		{"0 errors found", domain.SeverityError, "limitação: negação"},
		{"report generated without failure", domain.SeverityError, "limitação: negação"},
		{"retry succeeded after timeout", domain.SeverityError, "limitação: contexto"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := deriveSeverity(tt.message, domain.CategoryApplication, nil, domain.IngestionHTTP)
			if got != tt.want {
				t.Fatalf("deriveSeverity(%q) = %q, want %q %s", tt.message, got, tt.want, tt.note)
			}
		})
	}
}
