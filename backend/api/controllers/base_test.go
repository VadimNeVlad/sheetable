package controllers

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestEnsureApplicationDataDirectories(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "application-data")

	if err := ensureApplicationDataDirectories(dataRoot); err != nil {
		t.Fatalf("ensureApplicationDataDirectories returned error: %v", err)
	}

	for _, directory := range []string{
		dataRoot,
		filepath.Join(dataRoot, "sheets"),
		filepath.Join(dataRoot, "sheets", "uploaded-sheets"),
		filepath.Join(dataRoot, "sheets", "thumbnails"),
		filepath.Join(dataRoot, "composer"),
	} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("stat %s: %v", directory, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", directory)
		}
	}
}

func TestOpenDatabaseWithRetryRecoversFromTemporaryUnavailability(t *testing.T) {
	attempts := 0
	var delays []time.Duration

	database, err := openDatabaseWithRetry(
		5,
		10*time.Millisecond,
		25*time.Millisecond,
		func() (*gorm.DB, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("database unavailable")
			}
			return gorm.Open("sqlite3", ":memory:")
		},
		func(delay time.Duration) {
			delays = append(delays, delay)
		},
	)

	if err != nil {
		t.Fatalf("openDatabaseWithRetry returned error: %v", err)
	}
	if database == nil {
		t.Fatal("openDatabaseWithRetry returned a nil database")
	}
	defer database.Close()
	if err = database.DB().Ping(); err != nil {
		t.Fatalf("ping recovered database: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	wantDelays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	if !reflect.DeepEqual(delays, wantDelays) {
		t.Fatalf("delays = %v, want %v", delays, wantDelays)
	}
}

func TestOpenDatabaseWithRetryStopsAtConfiguredLimit(t *testing.T) {
	attempts := 0
	var delays []time.Duration

	_, err := openDatabaseWithRetry(
		4,
		10*time.Millisecond,
		25*time.Millisecond,
		func() (*gorm.DB, error) {
			attempts++
			return nil, errors.New("database unavailable")
		},
		func(delay time.Duration) {
			delays = append(delays, delay)
		},
	)

	if err == nil {
		t.Fatal("openDatabaseWithRetry returned nil error")
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want 4", attempts)
	}
	wantDelays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 25 * time.Millisecond}
	if !reflect.DeepEqual(delays, wantDelays) {
		t.Fatalf("delays = %v, want %v", delays, wantDelays)
	}
}
