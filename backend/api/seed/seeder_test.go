package seed

import (
	"testing"

	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestMigrateAndSeedIsIdempotent(t *testing.T) {
	database, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer database.Close()

	if err = MigrateAndSeed(database, "admin@example.test", "initial-password"); err != nil {
		t.Fatalf("first MigrateAndSeed: %v", err)
	}
	if err = MigrateAndSeed(database, "changed-admin@example.test", "changed-password"); err != nil {
		t.Fatalf("second MigrateAndSeed: %v", err)
	}

	var count int
	if err = database.Model(&models.User{}).Where("email = ?", "admin@example.test").Count(&count).Error; err != nil {
		t.Fatalf("count administrators: %v", err)
	}
	if count != 1 {
		t.Fatalf("administrator count = %d, want 1", count)
	}
}
