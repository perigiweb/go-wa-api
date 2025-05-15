package store

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/util/dbutil"
	"go.mau.fi/whatsmeow/types"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

const (
	broadcastTable          = "user_broadcasts"
	broadcastRecipientTable = "user_broadcast_recipients"
)

const (
	getBroadcastsQuery     = `SELECT * FROM ` + broadcastTable + ` WHERE user_id=$1 AND jid=$2 ORDER BY id DESC LIMIT $3 OFFSET $4`
	getCountBroadcastQuery = "SELECT COUNT(*) FROM " + broadcastTable + " WHERE user_id=$1 AND jid=$2"
	getBroadcastByIdQuery  = "SELECT * FROM " + broadcastTable + " WHERE id=$1"
	insertBroadcastQuery   = `INSERT INTO ` + broadcastTable + ` (
			user_id, jid, message, media, contact_type, contact_filter, filter_value, phones, campaign_name, sent_started_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id`
	updateBroadcastQuery = `UPDATE ` + broadcastTable + ` SET
			message=$1,
			media=$2,
			contact_type=$3,
			contact_filter=$4,
			filter_value=$5,
			phones=$6,
			campaign_name=$7,
			updated_at=$8,
			sent_started_at=$9
		WHERE id=$10`
	updateCompletedBroadcastQuery   = `UPDATE ` + broadcastTable + ` SET completed=$1, completed_at=$2, updated_at=$3 WHERE id=$4`
	deleteBroadcastQuery            = `DELETE FROM ` + broadcastTable + ` WHERE id=$1`
	getBroadcastRecipientsQuery     = "SELECT * FROM " + broadcastRecipientTable + " WHERE broadcast_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3"
	getBroadcastRecipientCountQuery = "SELECT COUNT(*) FROM " + broadcastRecipientTable + " WHERE broadcast_id=$1"
	insertBrodcastRecipientQuery    = `INSERT INTO ` + broadcastRecipientTable + ` (
			broadcast_id, name, phone
		) VALUES ($1, $2, $3) RETURNING id`
	updateSentStatusQuery = `UPDATE ` + broadcastRecipientTable + ` SET sent_status=$1, sent_at=$2, message_id=$3
	  WHERE id=$4`
	updateRecieptQuery       = `UPDATE ` + broadcastRecipientTable + ` SET sent_status=$1 WHERE message_id IN`
	getRandomBroadcastToSent = `SELECT * FROM ` + broadcastTable + ` WHERE sent_started_at <= now() AND completed=false ORDER BY RANDOM() LIMIT 1`
)

func convertJsonbToString(v []uint8) (d []string) {
	json.Unmarshal(v, &d)

	return d
}

func (r *Repo) ScanBroadcast(row dbutil.Scannable) (*entity.Broadcast, error) {
	var (
		id                                               int64
		broadcastUserId                                  int
		message, contactType, contactFilter, filterValue sql.NullString
		createdAt, completedAt, updatedAt, sentStartedAt sql.NullTime
		completed                                        bool
		campaignName                                     string
		phones                                           []uint8
		media                                            entity.UploadedFile
		broadcastJid                                     types.JID
	)

	err := row.Scan(
		&id,
		&broadcastUserId,
		&message,
		&media,
		&contactType,
		&contactFilter,
		&filterValue,
		&phones,
		&broadcastJid,
		&completed,
		&createdAt,
		&completedAt,
		&updatedAt,
		&campaignName,
		&sentStartedAt,
	)

	if err != nil {
		return nil, err
	}

	broadcast := entity.Broadcast{
		Id:            id,
		UserId:        broadcastUserId,
		Message:       message.String,
		Media:         &media,
		ContactType:   contactType.String,
		ContactFilter: contactFilter.String,
		FilterValue:   filterValue.String,
		Phones:        convertJsonbToString(phones),
		Jid:           broadcastJid,
		Completed:     completed,
		CreatedAt:     createdAt.Time,
		CampaignName:  campaignName,
		SentStartedAt: &sentStartedAt.Time,
	}

	if completedAt.Valid {
		broadcast.CompletedAt = &completedAt.Time
	}
	if updatedAt.Valid {
		broadcast.UpdatedAt = &updatedAt.Time
	}

	broadcast.Status = "pending"
	if broadcast.Completed && !completedAt.Valid {
		broadcast.Status = "pause"
	}
	if broadcast.Completed && completedAt.Valid {
		broadcast.Status = "complete"
	}

	return &broadcast, nil
}

func (r *Repo) GetBroadcasts(userId int, jid types.JID, limit int, offset int) ([]*entity.Broadcast, int, error) {
	broadcasts := make([]*entity.Broadcast, 0)
	totalBroadcast := 0

	err := r.db.QueryRow(getCountBroadcastQuery, userId, jid).Scan(&totalBroadcast)
	if err != nil {
		return broadcasts, totalBroadcast, err
	}

	if totalBroadcast == 0 {
		return broadcasts, totalBroadcast, nil
	}

	rows, err := r.db.Query(getBroadcastsQuery, userId, jid, limit, offset)
	if err != nil {
		return broadcasts, totalBroadcast, err
	}
	defer rows.Close()

	for rows.Next() {
		broadcast, scanErr := r.ScanBroadcast(rows)
		if scanErr == nil {
			broadcasts = append(broadcasts, broadcast)
		}
	}

	return broadcasts, totalBroadcast, nil
}

func (r *Repo) GetBroadcast(broadcastId int64) (*entity.Broadcast, error) {

	broadcast, err := r.ScanBroadcast(r.db.QueryRow(getBroadcastByIdQuery, broadcastId))
	if err != nil {
		return broadcast, err
	}

	device, _ := r.GetDeviceByJid(broadcast.Jid)

	broadcast.Device = device

	return broadcast, err
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
			broadcast.Jid,
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

func (r *Repo) UpdateRunningStatusBroadcast(broadcastId int64, status string) error {
	var completed bool
	if status == "pause" {
		completed = true
	} else {
		completed = false
	}
	log.Printf("Status: %s, Completed: %v, BroadcastId: %d", status, completed, broadcastId)
	_, err := r.db.Exec("UPDATE "+broadcastTable+" SET completed=$1 WHERE id=$2", completed, broadcastId)

	return err
}

func (r *Repo) UpdateCompletedBroadcast(broadcastId int64, completed bool, completedAt time.Time) error {
	_, err := r.db.Exec(updateCompletedBroadcastQuery, completed, completedAt, time.Now(), broadcastId)

	return err
}

func (r *Repo) GetBroadcastRecipients(broadcastId int64, limit int, offset int) ([]entity.BroadcastRecipient, int, error) {
	recipients := make([]entity.BroadcastRecipient, 0)
	total := 0

	err := r.db.QueryRow(getBroadcastRecipientCountQuery, broadcastId).Scan(&total)
	if err != nil {
		return recipients, total, err
	}

	rows, err := r.db.Query(getBroadcastRecipientsQuery, broadcastId, limit, offset)
	if err != nil {
		return recipients, total, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                    int
			rBroadcastId          int64
			phone, name           string
			sentStatus, messageId sql.NullString
			sentAt                sql.NullTime
		)

		if err := rows.Scan(
			&id,
			&rBroadcastId,
			&phone,
			&name,
			&sentStatus,
			&sentAt,
			&messageId,
		); err == nil {
			recipient := entity.BroadcastRecipient{
				Id:          id,
				BroadcastId: rBroadcastId,
				Phone:       phone,
				Name:        name,
				SentStatus:  sentStatus.String,
			}
			if sentAt.Valid {
				recipient.SentAt = &sentAt.Time
			}
			if messageId.Valid {
				recipient.MessageId = &messageId.String
			}

			recipients = append(recipients, recipient)
		} else {
			log.Printf("Rows Error: %v", err)
		}
	}

	return recipients, total, err
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

func (r *Repo) UpdateSentStatus(brId int, sentStatus string, messageId types.MessageID, sentAt time.Time) error {
	_, err := r.db.Exec(
		updateSentStatusQuery,
		sentStatus,
		sentAt,
		messageId,
		brId,
	)

	return err
}

func (r *Repo) UpdateBroadcastMessageReceipt(messageId []string, receipt string) error {
	args := make([]interface{}, len(messageId)+1)
	args[0] = receipt
	ins := make([]string, 0)
	for i, mId := range messageId {
		args[i+1] = mId
		ins = append(ins, "$"+strconv.Itoa(i+2))
	}

	q := updateRecieptQuery + ` (` + strings.Join(ins, ",") + `)`
	log.Printf("MessageID: %v, Receipt: %s >> Query: %s", messageId, receipt, q)

	affRows, err := r.db.Exec(
		q,
		args...,
	)

	log.Printf("Update Receipt: Rows Affected: %v Error: %v", affRows, err)

	return err
}

func (r *Repo) GetBroadcastToSend() (*entity.BroadcastToSend, error) {
	var (
		totalContact    int
		device          *entity.Device
		broadcastToSend entity.BroadcastToSend
	)

	broadcast, err := r.ScanBroadcast(r.db.QueryRow(getRandomBroadcastToSent))
	if err != nil {
		return nil, err
	}

	device, _ = r.GetDeviceByJid(broadcast.Jid)

	broadcast.Device = device

	broadcastToSend.Broadcast = *broadcast

	switch broadcast.ContactType {
	case "c":
		filterValue := broadcast.FilterValue
		filterField := contactFilterField(broadcast.ContactFilter)
		if filterField == "group" {
			cg := strings.Split(filterValue, ":")
			filterValue = cg[0]
			filterField = "id IN (SELECT contact_id FROM " + userContactGroupContactTableName + " WHERE group_id=$2)"
		} else {
			filterValue = broadcast.FilterValue + "%"
			if filterField == "" {
				filterField = "1=$2"
				filterValue = "1"
			} else {
				filterField = filterField + " ILIKE $2"
			}
		}
		countContactQuery := `SELECT count(*) FROM ` + userContactTableName + `
		WHERE in_wa=1 AND user_id=$1 AND ` + filterField + ` AND NOT EXISTS (
			SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$3 AND phone=user_contacts.phone
		)`
		err = r.db.QueryRow(
			countContactQuery,
			broadcast.UserId,
			filterValue,
			broadcast.Id,
		).Scan(&totalContact)
		if err != nil {
			return &broadcastToSend, err
		}

		broadcastToSend.TotalRecipient = totalContact
		if totalContact == 0 {
			return &broadcastToSend, nil
		}

		contactQuery := `SELECT name,phone,verified_name FROM ` + userContactTableName + `
		WHERE in_wa=1 AND user_id=$1 AND ` + filterField + ` AND NOT EXISTS (
			SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$3 AND phone=user_contacts.phone
		) ORDER BY RANDOM() LIMIT 1`
		//log.Printf("BroadcastId: %d, field: %s, value: %s, Query: %s", broadcastToSend.Broadcast.Id, filterField, filterValue, contactQuery)
		var contactName, contactPhone, contactVerifiedName sql.NullString

		err = r.db.QueryRow(
			contactQuery,
			broadcastToSend.Broadcast.UserId,
			filterValue,
			broadcastToSend.Broadcast.Id,
		).Scan(&contactName, &contactPhone, &contactVerifiedName)
		if err != nil {
			if err == sql.ErrNoRows {
				return &broadcastToSend, nil
			}
			return &broadcastToSend, err
		}

		broadcastToSend.Recipient = &entity.BroadcastRecipient{
			BroadcastId: broadcastToSend.Broadcast.Id,
			Phone:       contactPhone.String,
		}
		if contactVerifiedName.Valid {
			broadcastToSend.Recipient.Name = contactVerifiedName.String
		} else {
			broadcastToSend.Recipient.Name = contactName.String
		}
	case "w":
		device := &entity.Device{
			Jid:    &broadcastToSend.Broadcast.Jid,
			UserId: broadcastToSend.Broadcast.UserId,
		}

		device, err = r.GetDeviceByIdAndUserId(device)
		if err != nil {
			return &broadcastToSend, err
		}

		countContactQuery := `SELECT COUNT(*) FROM whatsmeow_contacts WHERE our_jid=$1 AND NOT EXISTS (
			SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$2 AND phone=whatsmeow_contacts.their_jid
		)`
		err = r.db.QueryRow(
			countContactQuery,
			device.Jid,
			broadcastToSend.Broadcast.Id,
		).Scan(&totalContact)
		if err != nil {
			return &broadcastToSend, err
		}
		broadcastToSend.TotalRecipient = totalContact
		if totalContact == 0 {
			return &broadcastToSend, nil
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
			if err == sql.ErrNoRows {
				return &broadcastToSend, nil
			}
			return &broadcastToSend, err
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

		countContactQuery := `WITH phones (name,phone) AS (VALUES ` + values + `)
		SELECT COUNT(*) FROM phones WHERE NOT EXISTS (
			SELECT 1 FROM "user_broadcast_recipients" WHERE broadcast_id=$1 AND phone=phones.phone
		)`
		err = r.db.QueryRow(
			countContactQuery,
			broadcastToSend.Broadcast.Id,
		).Scan(&totalContact)
		if err != nil {
			return &broadcastToSend, err
		}
		broadcastToSend.TotalRecipient = totalContact
		if totalContact == 0 {
			return &broadcastToSend, nil
		}

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
			if err == sql.ErrNoRows {
				return &broadcastToSend, nil
			}
			return &broadcastToSend, err
		}

		broadcastToSend.Recipient = &entity.BroadcastRecipient{
			BroadcastId: broadcastToSend.Broadcast.Id,
			Phone:       phone.String,
			Name:        name.String,
		}
	}

	return &broadcastToSend, nil
}

func (r *Repo) DeleteBroadcast(broadcastId int, jid *types.JID) error {
	var err error

	if jid != nil {
		_, err = r.db.Exec(deleteBroadcastQuery+" AND jid=$2", broadcastId, jid.ToNonAD())
	} else {
		_, err = r.db.Exec(deleteBroadcastQuery, broadcastId)
	}

	return err
}
