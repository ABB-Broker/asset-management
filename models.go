package main

import (
	"crypto/rand"
	"encoding/base32"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// Category is the GORM model for asset categories.
type Category struct {
	gorm.Model
	Name        string  `gorm:"not null"`
	Description string
	Assets      []Asset `gorm:"foreignKey:CategoryID"`
}

// Asset is the GORM model for physical assets.
type Asset struct {
	gorm.Model
	Name         string
	CategoryID   uint
	Category     Category `gorm:"constraint:OnDelete:CASCADE"`
	SerialNumber string
	PurchaseDate string
}

// Session stores an authenticated or pending-2FA user session in the database.
// DB-backed storage is required for Fiber Prefork mode where each worker is a
// separate OS process and cannot share in-memory state.
type Session struct {
	gorm.Model
	Token         string    `gorm:"uniqueIndex;not null"`
	Username      string    `gorm:"not null"`
	Authenticated bool      `gorm:"column:authenticated;default:false"`
	Pending2FA    bool      `gorm:"column:pending_2fa;default:true"`
	ExpiresAt     time.Time `gorm:"column:expires_at;index"`
}

func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
}
