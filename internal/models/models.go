// Package models contains all GORM database model types for the
// asset-management application.
package models

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// RoomPhotos is the GORM model for the table that contains photos of the rooms.
type RoomPhotos struct {
	gorm.Model
	RoomID   uint
	Name     string
	PhotoUrl string
	Room     Room `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`
}

// AssetPhotos is the GORM model for the table that contains photos of the assets.
type AssetPhotos struct {
	gorm.Model
	AssetID  uint
	Name     string
	PhotoUrl string
	Asset    Asset `gorm:"foreignKey:AssetID;references:ID;constraint:OnDelete:CASCADE"`
}

// Room is the GORM Model for rooms
type Room struct {
	gorm.Model
	RoomUUID    string
	RoomName    string `gorm:"not null"`
	Description string
	Assets      []Asset      `gorm:"foreignKey:RoomID"`
	RoomPhotos  []RoomPhotos `gorm:"foreignKey:RoomID"`
}

// BeforeCreate auto-generates a UUID v4 for any Room that does not already
// have one set (e.g. when created programmatically without going through the
// handler helpers).
func (r *Room) BeforeCreate(tx *gorm.DB) error {
	if r.RoomUUID == "" {
		r.RoomUUID = newUUID()
	}
	return nil
}

// Category is the GORM model for asset categories.
type Category struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	Assets      []Asset `gorm:"foreignKey:CategoryID"`
}

// Asset is the GORM model for physical assets.
type Asset struct {
	gorm.Model
	AssetUUID     string
	Name          string
	Description   string
	CategoryID    uint
	RoomID        uint
	SerialNumber  string
	PurchaseDate  string
	PurchasePrice uint
	Category      Category      `gorm:"foreignKey:CategoryID;references:ID;constraint:OnDelete:CASCADE"`
	Room          Room          `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`
	AssetPhotos   []AssetPhotos `gorm:"foreignKey:AssetID"`
}

// BeforeCreate auto-generates a UUID v4 for any Asset that does not already
// have one set.
func (a *Asset) BeforeCreate(tx *gorm.DB) error {
	if a.AssetUUID == "" {
		a.AssetUUID = newUUID()
	}
	return nil
}

// User is the GORM model for application user accounts (separate from the
// single hard-coded admin credential used for the login flow).
type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex;not null"`
	Email    string `gorm:"uniqueIndex"`
	Password string `gorm:"not null"`
	FullName string
	Role     string `gorm:"default:'viewer'"` // "admin" | "editor" | "viewer"
	Active   bool   `gorm:"default:true"`
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

// RandomToken generates a cryptographically random, Base-32-encoded token.
// Falls back to a Unix nanosecond timestamp string if rand.Read fails.
func RandomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
}

// newUUID generates a random UUID v4 string (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx).
// Kept package-private; external callers should use utils.NewUUID().
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("models.newUUID: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}
