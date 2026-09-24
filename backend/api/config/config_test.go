package config

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := NewConfig()
	assert.Equal(t, config.Environment, "development")
	assert.Equal(t, config.ApiSecret, "sheetable")
	assert.Equal(t, config.ConfigPath, "./config/")
	assert.Equal(t, config.AdminEmail, "admin@admin.com")
	assert.Equal(t, config.AdminPassword, "sheetable")
	assert.Equal(t, config.Database.Driver, "sqlite")
	assert.Equal(t, config.PDF2PNGURL, "http://localhost:5000/createthumbnail")
	assert.Equal(t, config.OpenOpusURL, "https://api.openopus.org")
}

func TestProductionConfigRejectsUnsafeDefaults(t *testing.T) {
	config := NewConfig()
	config.Environment = "production"

	err := config.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API_SECRET")
	assert.Contains(t, err.Error(), "ADMIN_PASSWORD")
	assert.Contains(t, err.Error(), "ADMIN_EMAIL")
}

func TestProductionConfigAcceptsNonDefaultCredentials(t *testing.T) {
	config := NewConfig()
	config.Environment = "production"
	config.ApiSecret = "0123456789abcdef0123456789abcdef"
	config.AdminEmail = "admin@example.test"
	config.AdminPassword = "a-long-unique-password"

	assert.NoError(t, config.Validate())
}

func TestConfigRejectsInvalidServiceURL(t *testing.T) {
	config := NewConfig()
	config.PDF2PNGURL = "pdf2png:5000"

	err := config.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PDF2PNG_URL")
}

func TestFileBackedSecretsOverrideEnvironmentValues(t *testing.T) {
	secretDirectory := t.TempDir()
	apiSecretFile := path.Join(secretDirectory, "api-secret")
	adminPasswordFile := path.Join(secretDirectory, "admin-password")
	if err := os.WriteFile(apiSecretFile, []byte("file-api-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adminPasswordFile, []byte("file-admin-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("API_SECRET", "environment-api-secret")
	t.Setenv("API_SECRET_FILE", apiSecretFile)
	t.Setenv("ADMIN_PASSWORD", "environment-admin-password")
	t.Setenv("ADMIN_PASSWORD_FILE", adminPasswordFile)

	config := ConfigBuilder().Build()

	assert.Equal(t, config.ApiSecret, "file-api-secret")
	assert.Equal(t, config.AdminPassword, "file-admin-password")
}

func TestPDF2PNGURLAcceptsServiceDNS(t *testing.T) {
	t.Setenv("PDF2PNG_URL", "http://pdf2png:5000/createthumbnail")

	config := ConfigBuilder().Build()

	assert.Equal(t, config.PDF2PNGURL, "http://pdf2png:5000/createthumbnail")
}

func TestEnvironmentVarzOverrideDefaults(t *testing.T) {
	os.Setenv("API_SECRET", "new secret")
	os.Setenv("ADMIN_PASSWORD", "password123")
	os.Setenv("DB_DRIVER", "mysql")
	os.Setenv("DB_PORT", "1234")
	defer os.Clearenv()
	config := Config()

	assert.Equal(t, config.ApiSecret, "new secret")
	assert.Equal(t, config.AdminPassword, "password123")
	assert.Equal(t, config.Database.Driver, "mysql")
	assert.Equal(t, config.Database.Port, 1234)
}

func TestDotEnvOverridesDefault(t *testing.T) {
	os.Setenv("ADMIN_EMAIL", "email set from environment variable")
	defer os.Clearenv()
	dotenvFile, err := ioutil.TempFile(".", ".test.*.env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dotenvFile.Name())
	_, err = dotenvFile.WriteString("ADMIN_EMAIL=email set from dotenv\nADMIN_PASSWORD=passwordSetFromDotenv")
	if err != nil {
		log.Fatal(err)
	}
	config := ConfigBuilder().WithDotenvFile(path.Join(".", dotenvFile.Name())).PanicOnMissingDotenv(true).Build()
	assert.Equal(t, config.AdminEmail, "email set from environment variable")
	assert.Equal(t, config.AdminPassword, "passwordSetFromDotenv")

}
