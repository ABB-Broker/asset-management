package models

import (
	"crypto/rand"
	"encoding/base32"
	"strconv"
	"time"

	"github.com/ABB-Broker/asset-management/internal/utils"
	"gorm.io/gorm"
)

type BaseModel struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

//
// LOCATION
//

type Location struct {
	LocationNo uint `gorm:"column:location_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	LocationUUID string
	LocationName string `gorm:"not null"`
	Description  string

	Assets         []Asset         `gorm:"foreignKey:LocationNo;"`
	LocationPhotos []LocationPhoto `gorm:"foreignKey:LocationNo;"`
}

type LocationPhoto struct {
	LocationPhotoNo uint `gorm:"column:location_photo_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	LocationNo uint `gorm:"column:location_no;type:int unsigned;index;not null"`

	Name     string
	PhotoUrl string

	Location *Location `gorm:"foreignKey:LocationNo;references:LocationNo;constraint:OnDelete:CASCADE"`
}

func (l *Location) BeforeCreate(tx *gorm.DB) error {
	if l.LocationUUID == "" {
		l.LocationUUID = utils.NewUUID()
	}
	return nil
}

//
// CATEGORY
//

type Category struct {
	CategoryNo uint `gorm:"column:category_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name        string `gorm:"not null"`
	Description string

	Assets []Asset `gorm:"foreignKey:CategoryNo;"`
}

//
// ASSET
//

type Asset struct {
	AssetNo uint `gorm:"column:asset_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	AssetUUID string

	Name        string
	Description string

	AssetType string `gorm:"type:enum('fixed','movable');not null;default:'fixed'"`

	CategoryNo uint  `gorm:"column:category_no;type:int unsigned;index;not null"`
	LocationNo *uint `gorm:"column:location_no;type:int unsigned;index"`

	SerialNumber  string
	PurchaseDate  string
	PurchasePrice uint

	Category *Category `gorm:"foreignKey:CategoryNo;references:CategoryNo;constraint:OnDelete:CASCADE"`
	Location *Location `gorm:"foreignKey:LocationNo;references:LocationNo;constraint:OnDelete:SET NULL"`

	AssetPhotos []AssetPhoto `gorm:"foreignKey:AssetNo;"`
	LendingLogs []LendingLog `gorm:"foreignKey:AssetNo;"`
	PICs        []PIC        `gorm:"foreignKey:AssetNo;"`
}

type AssetPhoto struct {
	AssetPhotoNo uint `gorm:"column:asset_photo_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	AssetNo uint `gorm:"column:asset_no;type:int unsigned;index;not null"`

	Name     string
	PhotoUrl string

	Asset *Asset `gorm:"foreignKey:AssetNo;references:AssetNo;constraint:OnDelete:CASCADE"`
}

func (a *Asset) BeforeCreate(tx *gorm.DB) error {
	if a.AssetUUID == "" {
		a.AssetUUID = utils.NewUUID()
	}
	return nil
}

//
// USER
// NOTE: AssigneeNo has been removed. To find a user's assignee record,
// query: WHERE user_no = ? on the assignees table.
//

type User struct {
	UserNo uint `gorm:"column:user_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Username string `gorm:"uniqueIndex;not null"`
	Email    string `gorm:"uniqueIndex"`

	Password string

	FullName    string
	PhoneNumber string
	Department  string
	Position    string

	EmployeeID string `gorm:"uniqueIndex"`

	Role string `gorm:"type:enum('admin','editor','viewer');default:'viewer'"`

	Active bool `gorm:"default:true"`
}

//
// PASSWORD TOKEN
//

type PasswordSetToken struct {
	PasswordSetTokenNo uint `gorm:"column:password_set_token_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Token string `gorm:"uniqueIndex;not null"`

	UserNo uint `gorm:"column:user_no;type:int unsigned;index;not null"`

	Kind string `gorm:"type:enum('invite','reset');not null;default:'invite'"`

	UsedAt *time.Time

	ExpiresAt time.Time `gorm:"index"`

	User *User `gorm:"foreignKey:UserNo;references:UserNo;constraint:OnDelete:CASCADE"`
}

func (p *PasswordSetToken) BeforeCreate(tx *gorm.DB) error {
	if p.Token == "" {
		p.Token = RandomToken()
	}

	if p.ExpiresAt.IsZero() {
		p.ExpiresAt = time.Now().Add(24 * time.Hour)
	}

	return nil
}

func (p *PasswordSetToken) IsValid() bool {
	return p.UsedAt == nil && time.Now().Before(p.ExpiresAt)
}

//
// EMAIL OTP
//

type EmailOTP struct {
	EmailOTPNo uint `gorm:"column:email_otp_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Code string `gorm:"not null"`

	UserNo uint `gorm:"column:user_no;type:int unsigned;index;not null"`

	UsedAt *time.Time

	ExpiresAt time.Time `gorm:"index"`

	User *User `gorm:"foreignKey:UserNo;references:UserNo;constraint:OnDelete:CASCADE"`
}

//
// ASSIGNEE
//

type Assignee struct {
	AssigneeNo uint `gorm:"column:assignee_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	AssigneeUUID string

	FullName string `gorm:"not null"`

	Email string `gorm:"uniqueIndex"`

	PhoneNumber string

	// UserNo links this assignee record to a system user.
	// This is the only FK between users and assignees (no reverse FK on users).
	UserNo *uint `gorm:"column:user_no;type:int unsigned;index"`

	Department string
	Position   string
	EmployeeID string

	Company string
	Notes   string

	User *User `gorm:"foreignKey:UserNo;references:UserNo;constraint:OnDelete:SET NULL"`

	LendingLogs []LendingLog `gorm:"foreignKey:AssigneeNo;"`
}

func (a *Assignee) BeforeCreate(tx *gorm.DB) error {
	if a.AssigneeUUID == "" {
		a.AssigneeUUID = utils.NewUUID()
	}
	return nil
}

func (a *Assignee) IsInternal() bool {
	return a.UserNo != nil
}

//
// LENDING LOG
//

type LendingLog struct {
	LendingLogNo uint `gorm:"column:lending_log_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	LendingUUID string

	AssetNo uint `gorm:"column:asset_no;type:int unsigned;index;not null"`

	AssigneeNo uint `gorm:"column:assignee_no;type:int unsigned;index;not null"`

	LentAt time.Time

	PlannedUseAt *time.Time
	ReturnedAt   *time.Time

	// Status flow: pending_signature → pending_approval → active → returned
	Status string `gorm:"type:enum('pending_signature','pending_approval','active','returned');default:'pending_signature'"`

	Notes string

	Asset *Asset `gorm:"foreignKey:AssetNo;references:AssetNo;constraint:OnDelete:CASCADE"`

	Assignee *Assignee `gorm:"foreignKey:AssigneeNo;references:AssigneeNo;constraint:OnDelete:CASCADE"`

	HandoverForm    *HandoverForm    `gorm:"foreignKey:LendingLogNo;references:LendingLogNo"`
	ApprovalRequest *ApprovalRequest `gorm:"foreignKey:LendingLogNo;references:LendingLogNo"`
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

//
// HANDOVER FORM
//

type HandoverForm struct {
	HandoverFormNo uint `gorm:"column:handover_form_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	FormUUID string

	LendingLogNo uint `gorm:"column:lending_log_no;type:int unsigned;uniqueIndex;not null"`

	FormToken string `gorm:"uniqueIndex"`

	SentAt   *time.Time
	SignedAt *time.Time

	SignatureData string `gorm:"type:longtext"`

	// Status: sent → signed (borrower signed, pending PIC approval) → published (PIC approved, receipt generated)
	Status string `gorm:"type:enum('sent','signed','published');default:'sent'"`

	ReceiptPath string

	LendingLog *LendingLog `gorm:"foreignKey:LendingLogNo;references:LendingLogNo;constraint:OnDelete:CASCADE"`
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

//
// APPROVAL REQUEST
//

type ApprovalRequest struct {
	ApprovalRequestNo uint `gorm:"column:approval_request_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	ApprovalUUID string

	LendingLogNo   uint `gorm:"column:lending_log_no;type:int unsigned;index;not null"`
	ApproverUserNo uint `gorm:"column:approver_user_no;type:int unsigned;index;not null"`

	ApprovalToken string `gorm:"uniqueIndex"`

	RequestedAt *time.Time
	DecidedAt   *time.Time

	// Status: pending → approved | rejected
	Status string `gorm:"type:enum('pending','approved','rejected');default:'pending'"`

	// Optional PIC signature on the approval page
	SignatureData string `gorm:"type:longtext"`

	Notes string

	LendingLog *LendingLog `gorm:"foreignKey:LendingLogNo;references:LendingLogNo;constraint:OnDelete:CASCADE"`
	Approver   *User       `gorm:"foreignKey:ApproverUserNo;references:UserNo;constraint:OnDelete:CASCADE"`
}

func (a *ApprovalRequest) BeforeCreate(tx *gorm.DB) error {
	if a.ApprovalUUID == "" {
		a.ApprovalUUID = utils.NewUUID()
	}
	if a.ApprovalToken == "" {
		a.ApprovalToken = RandomToken()
	}
	return nil
}

//
// NOTIFICATION
//

type Notification struct {
	NotificationNo uint `gorm:"column:notification_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// UserNo is the recipient of the notification.
	UserNo uint `gorm:"column:user_no;type:int unsigned;index;not null"`

	// Kind is a machine-readable event type string, e.g. "approval_requested".
	Kind  string `gorm:"not null"`
	Title string `gorm:"not null"`
	Body  string

	// ReferenceType + ReferenceNo let the UI deep-link to the relevant record.
	// e.g. ReferenceType="lending_log", ReferenceNo=42
	ReferenceType string
	ReferenceNo   *uint `gorm:"column:reference_no;type:int unsigned"`

	ReadAt *time.Time

	User *User `gorm:"foreignKey:UserNo;references:UserNo;constraint:OnDelete:CASCADE"`
}

func (n *Notification) IsRead() bool {
	return n.ReadAt != nil
}

//
// PIC
//

type PIC struct {
	PICNo uint `gorm:"column:pic_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	AssetNo uint `gorm:"column:asset_no;type:int unsigned;index;not null"`

	UserNo uint `gorm:"column:user_no;type:int unsigned;index;not null"`

	Notes string

	Asset *Asset `gorm:"foreignKey:AssetNo;references:AssetNo;constraint:OnDelete:CASCADE"`

	User *User `gorm:"foreignKey:UserNo;references:UserNo;constraint:OnDelete:CASCADE"`
}

//
// SESSION
//

type Session struct {
	SessionNo uint `gorm:"column:session_no;primaryKey;autoIncrement;type:int unsigned"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Token string `gorm:"uniqueIndex;not null"`

	UserNo uint `gorm:"column:user_no;type:int unsigned;index;not null"`

	Authenticated bool `gorm:"default:false"`

	Pending2FA bool `gorm:"column:pending_2fa;default:true"`

	ExpiresAt time.Time `gorm:"index"`

	User *User `gorm:"foreignKey:UserNo;references:UserNo;constraint:OnDelete:CASCADE"`
}

//
// HELPERS
//

func RandomToken() string {
	buf := make([]byte, 32)

	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	return base32.StdEncoding.
		WithPadding(base32.NoPadding).
		EncodeToString(buf)
}
