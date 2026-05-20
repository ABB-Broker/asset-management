// Package models contains all GORM database model types.
package models

import (
	"crypto/rand"
	"encoding/base32"
	"strconv"
	"time"

	"github.com/ABB-Broker/asset-management/internal/utils"
	"gorm.io/gorm"
)

// ─── Location (was Room) ──────────────────────────────────────────────────────

type LocationPhotos struct {
	gorm.Model
	LocationID uint
	Name       string
	PhotoUrl   string
	Location   Location `gorm:"foreignKey:LocationID;references:ID;constraint:OnDelete:CASCADE"`
}

type Location struct {
	gorm.Model
	LocationUUID   string
	LocationName   string `gorm:"not null"`
	Description    string
	Assets         []Asset          `gorm:"foreignKey:LocationID"`
	LocationPhotos []LocationPhotos `gorm:"foreignKey:LocationID"`
}

func (l *Location) BeforeCreate(tx *gorm.DB) error {
	if l.LocationUUID == "" {
		l.LocationUUID = utils.NewUUID()
	}
	return nil
}

// ─── Category ─────────────────────────────────────────────────────────────────

type Category struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Description string
	Assets      []Asset `gorm:"foreignKey:CategoryID"`
}

// ─── Asset ────────────────────────────────────────────────────────────────────

// AssetType values: "fixed" | "movable"
// Fixed assets: cannot be lent out (buildings, infrastructure, etc.)
// Movable assets: can be lent out and tracked via LendingLog

type AssetPhotos struct {
	gorm.Model
	AssetID  uint
	Name     string
	PhotoUrl string
	Asset    Asset `gorm:"foreignKey:AssetID;references:ID;constraint:OnDelete:CASCADE"`
}

type Asset struct {
	gorm.Model
	AssetUUID     string
	Name          string
	Description   string
	AssetType     string `gorm:"not null;default:'fixed'"` // "fixed" | "movable"
	CategoryID    uint
	LocationID    *uint
	SerialNumber  string
	PurchaseDate  string
	PurchasePrice uint

	Category    Category      `gorm:"foreignKey:CategoryID;references:ID;constraint:OnDelete:CASCADE"`
	Location    Location      `gorm:"foreignKey:LocationID;references:ID;constraint:OnDelete:CASCADE"`
	AssetPhotos []AssetPhotos `gorm:"foreignKey:AssetID"`
	LendingLogs []LendingLog  `gorm:"foreignKey:AssetID"`
}

func (a *Asset) BeforeCreate(tx *gorm.DB) error {
	if a.AssetUUID == "" {
		a.AssetUUID = utils.NewUUID()
	}
	return nil
}

// ─── User ─────────────────────────────────────────────────────────────────────

type User struct {
	gorm.Model
	Username    string `gorm:"uniqueIndex;not null"`
	Email       string `gorm:"uniqueIndex"`
	Password    string `gorm:"not null"`
	FullName    string
	PhoneNumber string
	Department  string
	Position    string // Job title / position
	EmployeeID  string `gorm:"uniqueIndex"`      // NIK / employee number
	Role        string `gorm:"default:'viewer'"` // "admin" | "editor" | "viewer"
	Active      bool   `gorm:"default:true"`
	AssigneeID  *uint  // FK back to Assignee (set after Assignee row is created)
}

// ─── Assignee ─────────────────────────────────────────────────────────────────
// Not a master table — represents anyone who can receive an asset.
// Internal employees have UserID set; external ones do not.

type Assignee struct {
	gorm.Model
	AssigneeUUID string
	FullName     string `gorm:"not null"`
	Email        string `gorm:"uniqueIndex"`
	PhoneNumber  string
	// For internal employees:
	UserID     *uint // nil for external assignees
	Department string
	Position   string
	EmployeeID string
	// For external assignees:
	Company string // e.g. insurance company name
	Notes   string
	// Relations
	User        *User        `gorm:"foreignKey:UserID"`
	LendingLogs []LendingLog `gorm:"foreignKey:AssigneeID"`
}

func (a *Assignee) BeforeCreate(tx *gorm.DB) error {
	if a.AssigneeUUID == "" {
		a.AssigneeUUID = utils.NewUUID()
	}
	return nil
}

// IsInternal returns true if this assignee is linked to an internal user.
func (a *Assignee) IsInternal() bool {
	return a.UserID != nil
}

// ─── LendingLog ───────────────────────────────────────────────────────────────
// Records each time a movable asset is lent to an assignee.
// Status: "active" | "returned" | "pending_signature"

type LendingLog struct {
	gorm.Model
	LendingUUID  string
	AssetID      uint
	AssigneeID   uint
	LentAt       time.Time
	ReturnedAt   *time.Time // nil while still lent out
	Status       string     `gorm:"default:'pending_signature'"` // pending_signature / active / returned
	Notes        string
	Asset        Asset         `gorm:"foreignKey:AssetID;constraint:OnDelete:CASCADE"`
	Assignee     Assignee      `gorm:"foreignKey:AssigneeID;constraint:OnDelete:CASCADE"`
	HandoverForm *HandoverForm `gorm:"foreignKey:LendingLogID"`
}

func (l *LendingLog) BeforeCreate(tx *gorm.DB) error {
	if l.LendingUUID == "" {
		l.LendingUUID = utils.NewUUID()
	}
	if l.LentAt.IsZero() {
		l.LentAt = time.Now()
	}
	return nil
}

// ─── HandoverForm ─────────────────────────────────────────────────────────────
// Tracks the digital signature form and the published Handover Receipt
// (Surat Serah Terima) for a lending event.
// Status: "sent" | "signed" | "published"

type HandoverForm struct {
	gorm.Model
	FormUUID      string
	LendingLogID  uint
	FormToken     string `gorm:"uniqueIndex"` // token embedded in the public form URL
	SentAt        *time.Time
	SignedAt      *time.Time
	SignatureData string `gorm:"type:text"`      // base64 PNG of the drawn signature
	Status        string `gorm:"default:'sent'"` // "sent" | "signed" | "published"
	// Path to the generated PDF receipt on disk (set after publishing)
	ReceiptPath string
	LendingLog  LendingLog `gorm:"foreignKey:LendingLogID;constraint:OnDelete:CASCADE"`
}

func (h *HandoverForm) BeforeCreate(tx *gorm.DB) error {
	if h.FormUUID == "" {
		h.FormUUID = utils.NewUUID()
	}
	if h.FormToken == "" {
		h.FormToken = RandomToken()
	}
	return nil
}

// ─── Session ──────────────────────────────────────────────────────────────────

type Session struct {
	gorm.Model
	Token         string    `gorm:"uniqueIndex;not null"`
	Username      string    `gorm:"not null"`
	Authenticated bool      `gorm:"column:authenticated;default:false"`
	Pending2FA    bool      `gorm:"column:pending_2fa;default:true"`
	ExpiresAt     time.Time `gorm:"column:expires_at;index"`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func RandomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
}
