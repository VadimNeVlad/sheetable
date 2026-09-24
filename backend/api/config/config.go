package config

import (
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
)

var (
	serverConfig ServerConfig
	configOnce   sync.Once
)

type configBuilder struct {
	dotenvFile           string
	errorOnMissingDotenv bool
}

func ConfigBuilder() configBuilder {
	return configBuilder{}
}

func (b configBuilder) WithDotenvFile(file string) configBuilder {
	b.dotenvFile = file
	return b
}

func (b configBuilder) PanicOnMissingDotenv(status bool) configBuilder {
	b.errorOnMissingDotenv = status
	return b
}

func (b configBuilder) Build() ServerConfig {
	serverConfig = NewConfig()

	dotenvFile := ".env"
	if b.dotenvFile != "" {
		dotenvFile = b.dotenvFile
	}
	dotenvFeeder := feeder.DotEnv{Path: dotenvFile}
	envFeeder := feeder.Env{}

	err := config.New().AddStruct(&serverConfig).AddFeeder(dotenvFeeder).Feed()
	if err != nil {
		if strings.Contains(err.Error(), "no such file") && b.errorOnMissingDotenv {
			log.Fatalf("error loading config from dotenv file %s: %s", dotenvFile, err.Error())
		}
	}
	err = config.New().AddStruct(&serverConfig).AddFeeder(envFeeder).Feed()
	if err != nil {
		log.Fatalf("error loding config from environemnt: %s", err.Error())
	}
	if err = applyFileSecrets(&serverConfig); err != nil {
		log.Fatalf("error loading file-backed secrets: %s", err.Error())
	}
	return serverConfig
}

func Config() ServerConfig {
	configOnce.Do(func() {
		serverConfig = ConfigBuilder().Build()
	})
	return serverConfig
}

type ServerConfig struct {
	Environment       string `env:"APP_ENV"`
	AdminEmail        string `env:"ADMIN_EMAIL"`
	AdminPassword     string `env:"ADMIN_PASSWORD"`
	AdminPasswordFile string `env:"ADMIN_PASSWORD_FILE"`
	ApiSecret         string `env:"API_SECRET"`
	ApiSecretFile     string `env:"API_SECRET_FILE"`
	ServerUrl         string `env:"SERVER_URL"`
	ConfigPath        string `env:"CONFIG_PATH"`
	PDF2PNGURL        string `env:"PDF2PNG_URL"`
	OpenOpusURL       string `env:"OPEN_OPUS_URL"`

	Dev  bool `env:"DEV"`
	Port int  `env:"PORT"`

	Database DatabaseConfig
	Smtp     SmtpConfig
}

// Bootstrap the application Config struct with the default config
func NewConfig() ServerConfig {
	return ServerConfig{
		Environment:   "development",
		AdminEmail:    "admin@admin.com",
		AdminPassword: "sheetable",
		ApiSecret:     "sheetable",
		ServerUrl:     "http://localhost:8080",
		ConfigPath:    "./config/",
		PDF2PNGURL:    "http://localhost:5000/createthumbnail",
		OpenOpusURL:   "https://api.openopus.org",
		Database: DatabaseConfig{
			Driver: "sqlite",
		},
		Smtp: SmtpConfig{
			Enabled: "0",
		},
	}
}

func (config ServerConfig) Validate() error {
	var problems []string

	environment := strings.ToLower(strings.TrimSpace(config.Environment))
	switch environment {
	case "development", "test", "production":
	default:
		problems = append(problems, "APP_ENV must be development, test, or production")
	}

	if config.Port < 0 || config.Port > 65535 {
		problems = append(problems, "PORT must be between 1 and 65535, or 0 for the default")
	}
	if strings.TrimSpace(config.ConfigPath) == "" {
		problems = append(problems, "CONFIG_PATH must not be empty")
	}

	validateAbsoluteHTTPURL := func(name string, value string) {
		parsedURL, err := url.ParseRequestURI(value)
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			problems = append(problems, fmt.Sprintf("%s must be an absolute http or https URL", name))
		}
	}
	validateAbsoluteHTTPURL("SERVER_URL", config.ServerUrl)
	validateAbsoluteHTTPURL("PDF2PNG_URL", config.PDF2PNGURL)
	validateAbsoluteHTTPURL("OPEN_OPUS_URL", config.OpenOpusURL)

	databaseDriver := strings.ToLower(strings.TrimSpace(config.Database.Driver))
	switch databaseDriver {
	case "sqlite":
	case "postgres", "mysql":
		if strings.TrimSpace(config.Database.Host) == "" {
			problems = append(problems, "DB_HOST must not be empty for a network database")
		}
		if strings.TrimSpace(config.Database.User) == "" {
			problems = append(problems, "DB_USER must not be empty for a network database")
		}
		if strings.TrimSpace(config.Database.Password) == "" {
			problems = append(problems, "DB_PASSWORD must not be empty for a network database")
		}
		if strings.TrimSpace(config.Database.Name) == "" {
			problems = append(problems, "DB_NAME must not be empty for a network database")
		}
		if config.Database.Port < 1 || config.Database.Port > 65535 {
			problems = append(problems, "DB_PORT must be between 1 and 65535 for a network database")
		}
	default:
		problems = append(problems, "DB_DRIVER must be sqlite, postgres, or mysql")
	}

	if config.Smtp.Enabled != "0" && config.Smtp.Enabled != "1" {
		problems = append(problems, "SMTP_ENABLED must be 0 or 1")
	}
	if config.Smtp.Enabled == "1" {
		if strings.TrimSpace(config.Smtp.From) == "" || strings.TrimSpace(config.Smtp.HostServerAddr) == "" {
			problems = append(problems, "SMTP_FROM and SMTP_SERVER_ADDR are required when SMTP is enabled")
		}
		if config.Smtp.HostServerPort < 1 || config.Smtp.HostServerPort > 65535 {
			problems = append(problems, "SMTP_HOST_SERVER_PORT must be between 1 and 65535 when SMTP is enabled")
		}
	}

	if environment == "production" {
		if config.Dev {
			problems = append(problems, "DEV must be disabled in production")
		}
		if len(config.ApiSecret) < 32 || config.ApiSecret == "sheetable" {
			problems = append(problems, "API_SECRET must be a non-default value of at least 32 characters in production")
		}
		if len(config.AdminPassword) < 12 || config.AdminPassword == "sheetable" {
			problems = append(problems, "ADMIN_PASSWORD must be a non-default value of at least 12 characters in production")
		}
		if config.AdminEmail == "admin@admin.com" {
			problems = append(problems, "ADMIN_EMAIL must not use the default production identity")
		}
		if _, err := mail.ParseAddress(config.AdminEmail); err != nil {
			problems = append(problems, "ADMIN_EMAIL must be a valid email address in production")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

type SmtpConfig struct {
	Enabled        string `env:"SMTP_ENABLED"`
	From           string `env:"SMTP_FROM"`
	HostServerAddr string `env:"SMTP_SERVER_ADDR"`
	HostServerPort int    `env:"SMTP_HOST_SERVER_PORT"`
	Username       string `env:"SMTP_USERNAME"`
	Password       string `env:"SMTP_PASSWORD"`
	PasswordFile   string `env:"SMTP_PASSWORD_FILE"`
}

type DatabaseConfig struct {
	Driver       string `env:"DB_DRIVER"`
	Host         string `env:"DB_HOST"`
	User         string `env:"DB_USER"`
	Password     string `env:"DB_PASSWORD"`
	PasswordFile string `env:"DB_PASSWORD_FILE"`
	Name         string `env:"DB_NAME"`
	Port         int    `env:"DB_PORT"`
}

func applyFileSecrets(config *ServerConfig) error {
	secrets := []struct {
		name     string
		filePath string
		target   *string
	}{
		{name: "API_SECRET", filePath: config.ApiSecretFile, target: &config.ApiSecret},
		{name: "ADMIN_PASSWORD", filePath: config.AdminPasswordFile, target: &config.AdminPassword},
		{name: "DB_PASSWORD", filePath: config.Database.PasswordFile, target: &config.Database.Password},
		{name: "SMTP_PASSWORD", filePath: config.Smtp.PasswordFile, target: &config.Smtp.Password},
	}

	for _, secret := range secrets {
		if secret.filePath == "" {
			continue
		}
		value, err := os.ReadFile(secret.filePath)
		if err != nil {
			return fmt.Errorf("read %s from %s: %w", secret.name, secret.filePath, err)
		}
		trimmedValue := strings.TrimRight(string(value), "\r\n")
		if trimmedValue == "" {
			return fmt.Errorf("%s file %s is empty", secret.name, secret.filePath)
		}
		*secret.target = trimmedValue
	}
	return nil
}
