package store

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
	"go.mau.fi/util/dbutil"
	"go.mau.fi/whatsmeow/types"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{
		db: db,
	}
}

func (r *Repo) InsertWAMessage(m entity.UserMessage) error {
	_, err := r.db.Exec(`INSERT INTO user_messages (
		id, their_jid, message, timestamp, device_id, from_me, type, push_name, receipt_type
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		m.ID, m.TheirJID, m.Message, m.Timestamp, m.DeviceId, m.FromMe, m.Type, m.PushName, m.ReceiptType)

	return err
}

func (r *Repo) UpdateWAMessageReceiptType(messageId []string, receiptType types.ReceiptType) error {
	if receiptType == types.ReceiptTypeDelivered {
		receiptType = "delivered"
	}
	args := make([]any, len(messageId)+1)
	args[0] = receiptType
	ins := make([]string, 0)
	for i, mId := range messageId {
		args[i+1] = mId
		ins = append(ins, "$"+strconv.Itoa(i+2))
	}

	q := `UPDATE user_messages SET receipt_type=$1 WHERE id IN (` + strings.Join(ins, ",") + `)`
	log.Printf("Update Message Receipt Type: args: %v, Receipt: %s >> Query: %s", args, receiptType, q)

	_, err := r.db.Exec(
		q,
		args...,
	)

	return err
}

func (r *Repo) DeleteWAMessages(messageId []string) error {
	args := make([]any, len(messageId)+1)
	ins := make([]string, 0)
	for i, mId := range messageId {
		args[i+1] = mId
		ins = append(ins, "$"+strconv.Itoa(i+2))
	}
	_, err := r.db.Exec("DELETE FROM user_messages WHERE id IN ("+strings.Join(ins, ",")+")", args...)

	return err
}

func (r *Repo) DeleteWAMessage(messageId string) error {
	ids := make([]string, 1)
	ids = append(ids, messageId)

	return r.DeleteWAMessages(ids)
}

type Chat struct {
	Message entity.UserMessage `json:"message"`
	From    types.ContactInfo  `json:"from"`
}

func (r *Repo) ScanChat(row dbutil.Scannable) (*entity.UserMessage, error) {
	var (
		id, deviceId, pushName, messageType, receiptType sql.NullString
		theirJid                                         types.JID
		message                                          entity.WAMessage
		timestamp                                        sql.NullTime
		fromMe                                           bool
	)

	err := row.Scan(
		&id,
		&theirJid,
		&message,
		&timestamp,
		&deviceId,
		&fromMe,
		&messageType,
		&pushName,
		&receiptType,
	)

	if err != nil {
		return nil, err
	}

	chat := entity.UserMessage{
		ID:        id.String,
		TheirJID:  &theirJid,
		Message:   &message,
		Timestamp: timestamp.Time,
		DeviceId:  deviceId.String,
		FromMe:    fromMe,
		Type:      receiptType.String,
		PushName:  pushName.String,
	}

	return &chat, nil
}

func (r *Repo) GetWAChats(deviceId string) ([]*entity.UserMessage, error) {
	var (
		chats []*entity.UserMessage
	)

	/*
		_ = r.db.QueryRow(`SELECT COUNT(*) FROM user_messages WHERE (their_jid, timestamp) IN (
			SELECT their_jid, max(timestamp) FROM user_messages WHERE device_id=$1 GROUP BY their_jid
		)`, deviceId).Scan(&total)

		log.Printf("GetWAChats: %s; Total: %d", deviceId, total)
	*/
	rows, err := r.db.Query(`SELECT * FROM user_messages WHERE (their_jid, timestamp) IN (
		SELECT their_jid, max(timestamp) FROM user_messages WHERE device_id=$1 GROUP BY their_jid
	) ORDER BY timestamp DESC`, deviceId)
	if err != nil {
		return chats, err
	}
	defer rows.Close()
	for rows.Next() {
		chat, err := r.ScanChat(rows)
		log.Printf("Err: %+v", err)
		if chat != nil {
			chats = append(chats, chat)
		}
	}

	log.Printf("Chats: %d", len(chats))

	return chats, err
}

func (r *Repo) GetWaConversation(deviceId string, theirJID types.JID, maxTimestamp time.Time) (messages []entity.UserMessage, err error) {
	var rows *sql.Rows

	if maxTimestamp.IsZero() {
		maxTimestamp = time.Now()
	}

	rows, err = r.db.Query(
		"SELECT * FROM user_messages WHERE device_id=$1 AND their_jid=$2 AND timestamp < $3 ORDER BY timestamp DESC",
		deviceId,
		theirJID,
		maxTimestamp,
	)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				id, device_id, push_name, message_type, receipt_type sql.NullString
				their_jid                                            types.JID
				message                                              entity.WAMessage
				timestamp                                            sql.NullTime
				from_me                                              bool
			)

			if err = rows.Scan(
				&id,
				&their_jid,
				&message,
				&timestamp,
				&device_id,
				&from_me,
				&message_type,
				&push_name,
				&receipt_type,
			); err == nil {
				m := entity.UserMessage{
					ID:        id.String,
					TheirJID:  &their_jid,
					Message:   &message,
					Timestamp: timestamp.Time,
					DeviceId:  device_id.String,
					FromMe:    from_me,
					Type:      message_type.String,
					PushName:  push_name.String,
				}

				messages = append(messages, m)
			}
		}
	}

	return
}
