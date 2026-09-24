package seed

import (
	"fmt"

	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/jinzhu/gorm"
)

func MigrateAndSeed(db *gorm.DB, email string, password string) error {
	err := db.AutoMigrate(&models.User{}, &models.Sheet{}, &models.Composer{}).Error
	if err != nil {
		return fmt.Errorf("migrate database schema: %w", err)
	}

	var existingUsers int
	if err = db.Model(&models.User{}).Count(&existingUsers).Error; err != nil {
		return fmt.Errorf("check initial administrator: %w", err)
	}
	if existingUsers > 0 {
		return nil
	}

	err = db.Model(&models.User{}).Create(&models.User{
		Email:    email,
		Password: password,
	}).Error
	if err != nil {
		return fmt.Errorf("create initial administrator: %w", err)
	}
	return nil
}
