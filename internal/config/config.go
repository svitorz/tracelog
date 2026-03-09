package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	Port int
}

type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

// DSN monta a string de conexão PostgreSQL para uso em adapters.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		d.Host,
		d.Port,
		d.Name,
		d.User,
		d.Password,
		d.SSLMode,
	)
}

// Load carrega configuração via ambiente e aplica defaults.
func Load() (Config, error) {
	httpPort, err := envInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}

	dbPort, err := envInt("DB_PORT", 5432)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		App: AppConfig{
			Name: envString("APP_NAME", "tracelog"),
			Env:  envString("APP_ENV", "local"),
		},
		HTTP: HTTPConfig{
			Port: httpPort,
		},
		DB: DBConfig{
			Host:     envString("DB_HOST", "localhost"),
			Port:     dbPort,
			Name:     envString("DB_NAME", "tracelog"),
			User:     envString("DB_USER", "postgres"),
			Password: envString("DB_PASSWORD", ""),
			SSLMode:  envString("DB_SSLMODE", "disable"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate aplica regras mínimas de consistência.
func (c Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("APP_NAME não pode ser vazio")
	}

	if strings.TrimSpace(c.App.Env) == "" {
		return fmt.Errorf("APP_ENV não pode ser vazio")
	}

	if err := validatePort(c.HTTP.Port, "HTTP_PORT"); err != nil {
		return err
	}

	if strings.TrimSpace(c.DB.Host) == "" {
		return fmt.Errorf("DB_HOST não pode ser vazio")
	}

	if err := validatePort(c.DB.Port, "DB_PORT"); err != nil {
		return err
	}

	if strings.TrimSpace(c.DB.Name) == "" {
		return fmt.Errorf("DB_NAME não pode ser vazio")
	}

	if strings.TrimSpace(c.DB.User) == "" {
		return fmt.Errorf("DB_USER não pode ser vazio")
	}

	if strings.TrimSpace(c.DB.SSLMode) == "" {
		return fmt.Errorf("DB_SSLMODE não pode ser vazio")
	}

	return nil
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
