package store

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

const (
	broadcastTable          = "user_broadcasts"
	broadcastRecipientTable = "user_broadcast_recipients"
)

const (
	getBroadcastsQuery = `
		SELECT * FROM ` + broadcastTable + ` WHERE user_id=$1 AND device_id=$2 ORDER BY id DESC LIMIT $3 OFFSET $4
	`
	getCountBroadcastQuery = "SELECT COUNT(*) FROM " + broadcastTable + " WHERE user_id=$1 AND device_id=$2"
	insertBroadcastQuery   = `
		INSERT INTO ` + broadcastTable + ` (
			user_id, device_id, message, media, contact_type, contact_filter, filter_value, phones, campaign_name, sent_started_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id
	`
	updateBroadcastQuery = `
		UPDATE ` + broadcastTable + ` SET
			message=$1,
			media=$2,
			contact_type=$3,
			contact_filter=$4,
			filter_value=$5,
			phones=$6,
			campaign_name=$7,
			updated_at=$8,
			sent_started_at=$9
		WHERE id=$10
	`
	updateCompletedBroadcastQuery = `
		UPDATE ` + broadcastTable + ` SET completed=$1, completed_at=$2, updated_at=$3 WHERE id=$4
	`
	deleteBroadcastQuery         = `DELETE FROM ` + broadcastTable + ` WHERE id=$1`
	insertBrodcastRecipientQuery = `
		INSERT INTO ` + broadcastRecipientTable + ` (
			broadcast_id, name, phone
		) VALUES ($1, $2, $3) RETURNING id
	`
	updateSentStatusQuery = `
		UPDATE ` + broadcastRecipientTable + ` SET sent_status=$1, sent_at=$2, message_id=$3 WHERE id=$4
	`
	getRandomBroadcastToSent = `SELECT * FROM ` + broadcastTable + ` WHERE sent_started_at <= now() AND completed=false`
)

func convertJsonbToString(v []uint8) (d []string) {
	json.Unmarshal(v, &d)

	return d
}

func (r *Repo) GetBroadcasts(userId int, deviceId string, limit int, offset int) ([]entity.Broadcast, int, error) {
	broadcasts := make([]entity.Broadcast, 0)
	totalBroadcast := 0

	err := r.db.QueryRow(getCountBroadcastQuery, userId, deviceId).Scan(&totalBroadcast)
	if err != nil {
		return broadcasts, totalBroadcast, err
	}

	if totalBroadcast == 0 {
		return broadcasts, totalBroadcast, nil
	}

	rows, err := r.db.Query(getBroadcastsQuery, userId, deviceId, limit, offset)
	if err != nil {
		return broadcasts, totalBroadcast, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                                                             int64
			user_id                                                        int
			message, contact_type, contact_filter, filter_value, device_id sql.NullString
			created_at, completed_at, updated_at, sent_started_at          sql.NullTime
			completed                                                      bool
			campaign_name                                                  string
			phones                                                         []uint8
			media                                                          entity.UploadedFile
		)

		if err := rows.Scan(
			&id,
			&user_id,
			&message,
			&media,
			&contact_type,
			&contact_filter,
			&filter_value,
			&phones,
			&device_id,
			&completed,
			&created_at,
			&completed_at,
			&updated_at,
			&campaign_name,
			&sent_started_at,
		); err == nil {
			broadcast := entity.Broadcast{
				Id:            id,
				UserId:        user_id,
				Message:       message.String,
				Media:         &media,
				ContactType:   contact_type.String,
				ContactFilter: contact_filter.String,
				FilterValue:   filter_value.String,
				Phones:        convertJsonbToString(phones),
				DeviceId:      device_id.String,
				Completed:     completed,
				CreatedAt:     created_at.Time,
				CompletedAt:   completed_at.Time,
				UpdatedAt:     updated_at.Time,
				CampaignName:  campaign_name,
			}

			broadcasts = append(broadcasts, broadcast)
		} else {
			log.Printf("Rows Error: %v", err)
		}
	}

	log.Printf("Rows Error: %v", rows.Err())

	return broadcasts, totalBroadcast, nil
}

func (r *Repo) SaveBroadcast(broadcast *entity.Broadcast) error {
	var err error

	if broadcast.Id != 0 {
		_, err = r.db.Exec(
			updateBroadcastQuery,
			broadcast.Message,
			broadcast.Media,
			broadcast.ContactType,
			broadcast.ContactFilter,
			broadcast.FilterValue,
			broadcast.Phones,
			broadcast.CampaignName,
			time.Now(),
			broadcast.SentStartedAt,
			broadcast.Id,
		)
	} else {
		err = r.db.QueryRow(
			insertBroadcastQuery,
			broadcast.UserId,
			broadcast.DeviceId,
			broadcast.Message,
			broadcast.Media,
			broadcast.ContactType,
			broadcast.ContactFilter,
			broadcast.FilterValue,
			broadcast.Phones,
			broadcast.CampaignName,
			broadcast.SentStartedAt,
		).Scan(&broadcast.Id)
	}

	return err
}

func (r *Repo) UpdateCompletedBroadcast(broadcastId int64, completed bool, completedAt time.Time) error {
	_, err := r.db.Exec(updateCompletedBroadcastQuery, completed, completedAt, time.Now(), broadcastId)

	return err
}

func (r *Repo) InsertBroadcastRecipient(br *entity.BroadcastRecipient) error {
	err := r.db.QueryRow(
		insertBrodcastRecipientQuery,
		br.BroadcastId,
		br.Name,
		br.Phone,
	).Scan(&br.Id)

	return err
}

func (r *Repo) UpdateSentStatus(brId int64, sentStatus string, messageId types.MessageID) error {
	sentAt := time.Now()

	_, err := r.db.Exec(
		updateSentStatusQuery,
		sentStatus,
		sentAt,
		messageId,
		brId,
	)

	return err
}

func (r *Repo) GetBroadcastToSend() (broadcastToSend entity.BroadcastToSend, err error) {
	var (
		id                                                             int64
		user_id                                                        int
		message, contact_type, contact_filter, filter_value, device_id sql.NullString
		created_at, completed_at, updated_at, sent_started_at          sql.NullTime
		completed                                                      bool
		campaign_name                                                  string
		phones                                                         []uint8
		media                                                          entity.UploadedFile
	)

	err = r.db.QueryRow(getRandomBroadcastToSent).Scan(
		&id,
		&user_id,
		&message,
		&media,
		&contact_type,
		&contact_filter,
		&filter_value,
		&phones,
		&device_id,
		&completed,
		&created_at,
		&completed_at,
		&updated_at,
		&campaign_name,
		&sent_started_at,
	)
	if err == nil {
		broadcastToSend.Broadcast = entity.Broadcast{
			Id:            id,
			UserId:        user_id,
			Message:       message.String,
			Media:         &media,
			ContactType:   contact_type.String,
			ContactFilter: contact_filter.String,
			FilterValue:   filter_value.String,
			Phones:        convertJsonbToString(phones),
			DeviceId:      device_id.String,
			Completed:     completed,
			CreatedAt:     created_at.Time,
			CompletedAt:   completed_at.Time,
			UpdatedAt:     updated_at.Time,
			CampaignName:  campaign_name,
			SentStartedAt: sent_started_at.Time,
		}

		switch broadcastToSend.Broadcast.ContactType {
		case "c":
			filterField := contactFilterField(broadcastToSend.Broadcast.ContactFilter)
			filterValue := broadcastToSend.Broadcast.FilterValue + "%"
			if filterField == "" {
				filterField = "1"
				filterValue = "1"
			}
			contactQuery := `SELECT name,phone,verified_name FROM ` + userContactTableName + `
			WHERE in_wa=1 AND user_id=$1 AND ` + filterField + ` ILIKE $2 AND NOT EXISTS (
				SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$3 AND phone=user_contacts.phone
			) ORDER BY RANDOM() LIMIT 1`
			log.Printf("BroadcastId: %d, field: %s, value: %s, Query: %s", broadcastToSend.Broadcast.Id, filterField, filterValue, contactQuery)
			var name, phone, verified_name sql.NullString

			err = r.db.QueryRow(
				contactQuery,
				broadcastToSend.Broadcast.UserId,
				filterValue,
				broadcastToSend.Broadcast.Id,
			).Scan(&name, &phone, &verified_name)
			if err != nil {
				return broadcastToSend, err
			}

			broadcastToSend.Recipient.BroadcastId = broadcastToSend.Broadcast.Id
			broadcastToSend.Recipient.Phone = phone.String
			if verified_name.Valid {
				broadcastToSend.Recipient.Name = verified_name.String
			} else {
				broadcastToSend.Recipient.Name = name.String
			}
		case "w":
			device := &entity.Device{
				Id:     broadcastToSend.Broadcast.DeviceId,
				UserId: broadcastToSend.Broadcast.UserId,
			}

			device, err = r.GetDeviceByIdAndUserId(device)
			if err != nil {
				return broadcastToSend, err
			}

			contactQuery := `SELECT full_name, their_jid FROM whatsmeow_contacts WHERE our_jid=$1 AND NOT EXISTS (
				SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$2 AND phone=whatsmeow_contacts.their_jid
			)`

			var name, phone sql.NullString

			err = r.db.QueryRow(
				contactQuery,
				device.Jid,
				broadcastToSend.Broadcast.Id,
			).Scan(&name, &phone)
			if err != nil {
				return broadcastToSend, err
			}

			broadcastToSend.Recipient = &entity.BroadcastRecipient{
				BroadcastId: broadcastToSend.Broadcast.Id,
				Phone:       phone.String,
				Name:        name.String,
			}
		case "p":
			phones := make([]string, 0)
			for _, p := range broadcastToSend.Broadcast.Phones {
				phones = append(phones, `('', '`+p+`')`)
			}
			values := strings.Join(phones, ",")
			contactQuery := `WITH phones (name,phone) AS (VALUES ` + values + `)
			SELECT name,phone FROM phones WHERE NOT EXISTS (
				SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$1 AND phone=phones.phone
			)`

			var name, phone sql.NullString

			err = r.db.QueryRow(
				contactQuery,
				broadcastToSend.Broadcast.Id,
			).Scan(&name, &phone)
			if err != nil {
				log.Printf("\n >> Error Type: %v\n", err)
				if err == sql.ErrNoRows {
					return broadcastToSend, nil
				}
				return broadcastToSend, err
			}

			broadcastToSend.Recipient = &entity.BroadcastRecipient{
				BroadcastId: broadcastToSend.Broadcast.Id,
				Phone:       phone.String,
				Name:        name.String,
			}
		}
	}

	log.Printf("\n >> Error Type: %T\n", err)

	return broadcastToSend, err
}

func (r *Repo) DeleteBroadcast(broadcastId int, deviceId string) error {
	var err error

	if deviceId != "" {
		_, err = r.db.Exec(deleteBroadcastQuery+" AND deviceId=$2", broadcastId, deviceId)
	} else {
		_, err = r.db.Exec(deleteBroadcastQuery, broadcastId)
	}

	return err
}
