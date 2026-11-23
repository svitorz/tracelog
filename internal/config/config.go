package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultAppName  = "tracelog"
	defaultAppEnv   = "local"
	defaultHTTPPort = 8080
	// Só localhost por padrão: a API de leitura expõe payloads sem autenticação (spec 0005).
	defaultHTTPHost = "127.0.0.1"
	// 1 MiB (spec 0002, seção 5).
	defaultHTTPMaxBodyBytes = 1 << 20
	defaultDBHost           = "localhost"
	defaultDBPort           = 5432
	defaultDBSSLMode        = "disable"
)

// Config concentra toda configuração da aplicação.
type Config struct {
	App  AppConfig
	HTTP HTTPConfig
	DB   DBConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Host         string
	Port         int
	MaxBodyBytes int64
}

type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

// DSN monta a string de conexão PostgreSQL para uso em adapters. Os valores vão entre
// aspas simples, com \ e ' escapados (regra do libpq), para que uma senha com espaço
// ou aspas não quebre a string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		quoteDSNValue(d.Host),
		d.Port,
		quoteDSNValue(d.Name),
		quoteDSNValue(d.User),
		quoteDSNValue(d.Password),
		quoteDSNValue(d.SSLMode),
	)
}

var dsnValueEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

func quoteDSNValue(value string) string {
	return "'" + dsnValueEscaper.Replace(value) + "'"
}

// Load carrega configuração via ambiente e aplica defaults.
func Load() (Config, error) {
	if err := loadDotEnvIfPresent(); err != nil {
		return Config{}, err
	}

	httpPort, err := envInt("HTTP_PORT", defaultHTTPPort)
	if err != nil {
		return Config{}, err
	}

	maxBodyBytes, err := envInt("HTTP_MAX_BODY_BYTES", defaultHTTPMaxBodyBytes)
	if err != nil {
		return Config{}, err
	}

	dbPort, err := envInt("DB_PORT", defaultDBPort)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		App: AppConfig{
			Name: envString("APP_NAME", defaultAppName),
			Env:  envString("APP_ENV", defaultAppEnv),
		},
		HTTP: HTTPConfig{
			Host:         envString("HTTP_HOST", defaultHTTPHost),
			Port:         httpPort,
			MaxBodyBytes: int64(maxBodyBytes),
		},
		DB: DBConfig{
			Host:     envString("DB_HOST", defaultDBHost),
			Port:     dbPort,
			Name:     requiredEnvString("DB_NAME"),
			User:     requiredEnvString("DB_USER"),
			Password: requiredEnvString("DB_PASSWORD"),
			SSLMode:  envString("DB_SSLMODE", defaultDBSSLMode),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate aplica regras mínimas de consistência.
func (c Config) Validate() error {
	var validationErrors []string

	if strings.TrimSpace(c.App.Name) == "" {
		validationErrors = append(validationErrors, "APP_NAME cannot be empty")
	}

	if strings.TrimSpace(c.App.Env) == "" {
		validationErrors = append(validationErrors, "APP_ENV cannot be empty")
	}

	if !isValidHost(c.HTTP.Host) {
		validationErrors = append(validationErrors, "HTTP_HOST must be an IP address or a host name")
	}

	if err := validatePort(c.HTTP.Port, "HTTP_PORT"); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	if c.HTTP.MaxBodyBytes <= 0 {
		validationErrors = append(validationErrors, "HTTP_MAX_BODY_BYTES must be a positive number of bytes")
	}

	if strings.TrimSpace(c.DB.Host) == "" {
		validationErrors = append(validationErrors, "DB_HOST cannot be empty")
	}

	if err := validatePort(c.DB.Port, "DB_PORT"); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	if strings.TrimSpace(c.DB.Name) == "" {
		validationErrors = append(validationErrors, "DB_NAME is required")
	}

	if strings.TrimSpace(c.DB.User) == "" {
		validationErrors = append(validationErrors, "DB_USER is required")
	}

	if strings.TrimSpace(c.DB.Password) == "" {
		validationErrors = append(validationErrors, "DB_PASSWORD is required")
	}

	if strings.TrimSpace(c.DB.SSLMode) == "" {
		validationErrors = append(validationErrors, "DB_SSLMODE cannot be empty")
	} else if !isValidSSLMode(c.DB.SSLMode) {
		validationErrors = append(validationErrors, "DB_SSLMODE must be one of disable, allow, prefer, require, verify-ca, verify-full")
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(validationErrors, "; "))
	}

	return nil
}

var hostNamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)

func isValidHost(host string) bool {
	return net.ParseIP(host) != nil || (len(host) <= 253 && hostNamePattern.MatchString(host))
}

func validatePort(port int, key string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s deve estar entre 1 e 65535", key)
	}

	return nil
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func requiredEnvString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s inválido: %w", key, err)
	}

	return port, nil
}

func isValidSSLMode(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

func loadDotEnvIfPresent() error {
	path := filepath.Clean(".env")

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("erro ao abrir .env: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("linha inválida no .env (%d)", lineNumber)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return fmt.Errorf("chave vazia no .env (%d)", lineNumber)
		}

		value = strings.Trim(value, "\"'")

		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("erro ao definir %s via .env: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("erro ao ler .env: %w", err)
	}

	return nil
}
