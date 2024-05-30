package entity

import (
	"encoding/json"
	"errors"
	"time"

	"database/sql/driver"

	"go.mau.fi/whatsmeow/types"
)

type UploadedFile struct {
	Data string `json:"data"`
	Name string `json:"name"`
	Size int    `json:"size"`
	Mime string `json:"mime"`
}

func (f UploadedFile) Value() (driver.Value, error) {
	return json.Marshal(f)
}

func (f *UploadedFile) Scan(v any) error {
	if v == nil {
		f.Data, f.Name, f.Size, f.Mime = "", "", 0, ""
		return nil
	}

	b, ok := v.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &f)
}

type User struct {
	Id       int    `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `db:"password"`
}

type UserSession struct {
	Id        string `json:"id"`
	UserId    int    `json:"userId"`
	Status    bool   `json:"status"`
	UserAgent string `json:"userAgent"`
	IpAddress string `json:"ip"`
	ExpiredAt int64  `json:"expiredAt"`
}

type UserContact struct {
	Id           int    `json:"id"`
	UserId       int    `json:"userId"`
	Name         string `json:"name" validate:"required"`
	Phone        string `json:"phone" validate:"required"`
	InWA         int    `json:"inWA"`
	VerifiedName string `json:"verifiedName"`
}

type Device struct {
	Id        string     `json:"id"`
	UserId    int        `json:"userId"`
	Name      string     `json:"name"`
	Jid       *types.JID `json:"jid"`
	Connected bool       `json:"connected"`
}

type Broadcast struct {
	Id            int64         `json:"id"`
	UserId        int           `json:"user_id"`
	DeviceId      string        `json:"deviceId"`
	Message       string        `json:"message" validate:"required,min=100"`
	Media         *UploadedFile `json:"media"`
	ContactType   string        `json:"contactType" validate:"required"`
	ContactFilter string        `json:"contactFilter"`
	FilterValue   string        `json:"filterValue"`
	Phones        []string      `json:"phones"`
	Completed     bool          `json:"completed"`
	CreatedAt     time.Time     `json:"createdAt"`
	CompletedAt   time.Time     `json:"completedAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	CampaignName  string        `json:"campaignName" validate:"required"`
	SentStartedAt time.Time     `json:"sentStartedAt"`
}

type BroadcastRecipient struct {
	Id          int              `json:"id"`
	BroadcastId int64            `json:"broadcastId"`
	Phone       string           `json:"phone"`
	Name        string           `json:"name"`
	SentStatus  string           `json:"sentStatus"`
	SentAt      *time.Time       `json:"sentAt"`
	MessageId   *types.MessageID `json:"messageId"`
}

type BroadcastToSend struct {
	Broadcast Broadcast
	Recipient *BroadcastRecipient
}
