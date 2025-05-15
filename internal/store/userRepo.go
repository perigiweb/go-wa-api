package store

import (
	"database/sql"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/perigiweb/go-wa-api/internal"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

const (
	userTableName                    = "users"
	userSessionTableName             = "user_sessions"
	userContactTableName             = "user_contacts"
	userContactGroupTableName        = "user_contact_groups"
	userContactGroupContactTableName = "user_contact_groups_contacts"
)

func (r *Repo) SaveUser(user *entity.User) error {
	args := make([]any, 0)
	args = append(args, sql.Named("name", user.Name))
	args = append(args, sql.Named("email", user.Email))
	if user.Id != 0 {
		q := "UPDATE " + userTableName + " SET name=@name, email=@email, updated_at=@updated_at"
		if user.Password != "" {
			q = q + ", password=@password"
			pass, err := internal.PasswordHash(user.Password)
			if err != nil {
				return err
			}
			args = append(args, sql.Named("password", pass))
		}
		q = q + " WHERE id=@id"
		args = append(args, sql.Named("updated_at", time.Now()))
		args = append(args, sql.NamedArg{Name: "id", Value: user.Id})
		_, err := r.db.Exec(q, args...)
		if err != nil {
			return err
		}
	} else {
		pass, err := internal.PasswordHash(user.Password)
		if err != nil {
			return err
		}
		args = append(args, sql.Named("password", pass))
		args = append(args, sql.Named("registered_at", time.Now()))
		args = append(args, sql.Named("updated_at", time.Now()))
		_, err = r.db.Exec(`INSERT INTO `+userTableName+` (
			name, email, password, registered_at, updated_at
		) VALUES (@name, @email, @password, @registered_at, @updated_at)`, args...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Repo) GetUserById(userId int) (user entity.User, err error) {
	err = r.db.QueryRow("SELECT id,email,name,password FROM "+userTableName+" WHERE id=$1", userId).Scan(
		&user.Id,
		&user.Email,
		&user.Name,
		&user.Password,
	)

	return user, err
}

func (r *Repo) GetUserByEmail(userEmail string) (user entity.User, err error) {
	err = r.db.QueryRow("SELECT id,email,name,password FROM "+userTableName+" WHERE email=$1", userEmail).Scan(
		&user.Id,
		&user.Email,
		&user.Name,
		&user.Password,
	)

	return user, err
}

func (r *Repo) InsertNewUserSession(userId int, userAgent string, ipAddress string) (userSession entity.UserSession, err error) {
	userSession.UserId = userId
	userSession.UserAgent = userAgent
	userSession.IpAddress = ipAddress
	userSession.Status = true
	userSession.ExpiredAt = time.Now().Add(time.Hour * 24 * 30).Unix()

	var q = "INSERT INTO " + userSessionTableName + " (user_id, status, user_agent, ip, expired_at) VALUES ($1, $2, $3, $4, $5) RETURNING id"
	err = r.db.QueryRow(q, userSession.UserId, userSession.Status, userSession.UserAgent, userSession.IpAddress, userSession.ExpiredAt).Scan(&userSession.Id)

	return userSession, err
}

func (r *Repo) GetUserSessionById(sessionId string) (userSession entity.UserSession, err error) {
	var q = "SELECT id, user_id, status, user_agent, ip, expired_at FROM " + userSessionTableName + " WHERE id=$1"
	err = r.db.QueryRow(q, sessionId).Scan(
		&userSession.Id,
		&userSession.UserId,
		&userSession.Status,
		&userSession.UserAgent,
		&userSession.IpAddress,
		&userSession.ExpiredAt,
	)

	return userSession, err
}

func (r *Repo) DeleteUserSessionById(sessionId string) error {
	_, err := r.db.Exec("DELETE FROM "+userSessionTableName+" WHERE id=$1", sessionId)

	return err
}

func (r *Repo) GetTotalUserContacts(userId int, f string, v string) (int, error) {
	var err error
	totalContact := 0

	filterField := contactFilterField(f)
	query := "SELECT COUNT(id) FROM " + userContactTableName + " WHERE in_wa=1 AND user_id=$1"
	if filterField == "group" {
		cg := strings.Split(v, ":")
		cgId, _ := strconv.Atoi(cg[0])
		err = r.db.QueryRow(query+" AND id IN (SELECT contact_id FROM "+userContactGroupContactTableName+" WHERE group_id=$2)", userId, cgId).Scan(&totalContact)
	} else if filterField != "" && v != "" {
		err = r.db.QueryRow(query+" AND "+filterField+" ILIKE $2", userId, v+"%").Scan(&totalContact)
	} else {
		err = r.db.QueryRow(query, userId).Scan(&totalContact)
	}

	return totalContact, err
}

func (r *Repo) GetUserContacts(userId int, limit int, offset int, s string) ([]entity.UserContact, int, error) {
	contacts := make([]entity.UserContact, 0)
	totalContact := 0

	f := ""
	if s != "" {
		var searchTerm, inWA, group string
		terms := strings.Split(s, "--")
		for _, term := range terms {
			term = strings.TrimSpace(term)
			if strings.Contains(term, "=") {
				params := strings.Split(term, "=")
				if params[0] == "wa" {
					inWA = strings.TrimSpace(params[1])
				}
				if params[0] == "group" {
					group = strings.TrimSpace(params[1])
				}
			} else {
				if searchTerm != "" {
					searchTerm = searchTerm + " " + term
				} else {
					searchTerm = term
				}
			}
		}

		f = " AND (name ILIKE '%" + searchTerm + "%' OR phone ILIKE '%" + searchTerm + "%')"
		if inWA == "0" || inWA == "1" {
			f = f + " AND in_wa = " + inWA
		}
		if group != "" {
			f = f + ` AND id IN (SELECT contact_id
				FROM ` + userContactGroupContactTableName + ` a, ` + userContactGroupTableName + ` b
				WHERE a.group_id=b.id AND b.name='` + group + `')`
		}
	}

	err := r.db.QueryRow("SELECT COUNT(id) FROM "+userContactTableName+" WHERE user_id=$1"+f, userId).Scan(&totalContact)
	if err != nil {
		return contacts, totalContact, err
	}

	if totalContact == 0 {
		return contacts, totalContact, nil
	}

	q := "SELECT id,user_id,name,phone,in_wa,verified_name FROM " + userContactTableName + " WHERE user_id=$1" + f + " ORDER BY in_wa DESC, name ASC LIMIT $2 OFFSET $3"
	rows, err := r.db.Query(q, userId, limit, offset)
	if err != nil {
		return contacts, totalContact, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, inWa                  int
			contactUserId             sql.NullInt64
			name, phone, verifiedName sql.NullString
		)

		if err := rows.Scan(
			&id,
			&contactUserId,
			&name,
			&phone,
			&inWa,
			&verifiedName,
		); err == nil {
			contact := entity.UserContact{
				Id:           id,
				UserId:       int(contactUserId.Int64),
				Name:         name.String,
				Phone:        phone.String,
				InWA:         inWa,
				VerifiedName: verifiedName.String,
				Groups:       r.GetContactGroupByContactId(id),
			}
			contacts = append(contacts, contact)
		}
	}

	return contacts, totalContact, nil
}

func (r *Repo) GetUserContactByFilter(userId int, f string, v string, limit int) ([]entity.UserContact, error) {
	var (
		rows *sql.Rows
		err  error
	)
	contacts := make([]entity.UserContact, 0)

	filterField := contactFilterField(f)
	if filterField == "" {
		return contacts, errors.New("no field for filter: " + f)
	}

	query := `SELECT id,user_id,name,phone,in_wa,verified_name FROM ` + userContactTableName + `
		WHERE in_wa=1 AND user_id=$1`
	if filterField == "group" {
		cg := strings.Split(v, ":")
		cgId, _ := strconv.Atoi(cg[0])
		rows, err = r.db.Query(query+" AND id IN (SELECT contact_id FROM "+userContactGroupContactTableName+" WHERE group_id=$2) LIMIT $3", userId, cgId, limit)
	} else {
		rows, err = r.db.Query(query+"AND "+filterField+"=$2 LIMIT $3", userId, v+"%", limit)
	}

	if err != nil {
		return contacts, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, in_wa                  int
			user_id                    sql.NullInt64
			name, phone, verified_name sql.NullString
		)

		if err := rows.Scan(
			&id,
			&user_id,
			&name,
			&phone,
			&in_wa,
			&verified_name,
		); err == nil {
			contact := entity.UserContact{
				Id:           id,
				UserId:       int(user_id.Int64),
				Name:         name.String,
				Phone:        phone.String,
				InWA:         in_wa,
				VerifiedName: verified_name.String,
				Groups:       r.GetContactGroupByContactId(id),
			}
			contacts = append(contacts, contact)
		}
	}

	return contacts, nil
}

func (r *Repo) GetContactGroupByContactId(contactId int) []string {
	groups := make([]string, 0)
	rows, err := r.db.Query("SELECT name FROM "+userContactGroupTableName+"  WHERE id IN (SELECT group_id FROM "+userContactGroupContactTableName+" WHERE contact_id=$1)", contactId)

	if err != nil {
		log.Println(err.Error())
		return groups
	}
	defer rows.Close()

	var groupName string
	for rows.Next() {
		if e := rows.Scan(&groupName); e == nil {
			groups = append(groups, groupName)
		}
	}

	return groups
}

func (r *Repo) GetUserContactGroups(userId int) ([]entity.UserContactGroup, error) {
	contactGroups := make([]entity.UserContactGroup, 0)

	rows, err := r.db.Query("SELECT id, user_id, name FROM "+userContactGroupTableName+" WHERE user_id=$1", userId)
	if err != nil {
		return contactGroups, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id      int
			user_id sql.NullInt64
			name    sql.NullString
		)
		if e := rows.Scan(&id, &user_id, &name); e == nil {
			contactGroup := entity.UserContactGroup{
				Id:     id,
				UserId: int(user_id.Int64),
				Name:   name.String,
			}

			contactGroups = append(contactGroups, contactGroup)
		}
	}

	return contactGroups, err
}

func (r *Repo) SaveUserContact(contact *entity.UserContact) (err error) {
	if contact.Id != 0 {
		_, err = r.db.Exec(
			"UPDATE "+userContactTableName+" SET name=$1, phone=$2, in_wa=$3, verified_name=$4 WHERE id=$5",
			contact.Name,
			contact.Phone,
			contact.InWA,
			contact.VerifiedName,
			contact.Id,
		)
	} else {
		err = r.db.QueryRow(
			"INSERT INTO "+userContactTableName+" (user_id, name, phone, in_wa, verified_name) VALUES ($1, $2, $3, $4, $5) ON CONFLICT ON CONSTRAINT user_contacts_phone_user DO UPDATE SET name=$6, phone=$7, verified_name=$8 RETURNING id",
			contact.UserId,
			contact.Name,
			contact.Phone,
			contact.InWA,
			contact.VerifiedName,
			contact.Name,
			contact.Phone,
			contact.VerifiedName,
		).Scan(&contact.Id)
	}

	if err == nil && len(contact.Groups) > 0 {
		_, _ = r.db.Exec("DELETE FROM "+userContactGroupContactTableName+" WHERE contact_id=$1", contact.Id)

		var contactGroupId int64
		for _, groupName := range contact.Groups {
			gErr := r.db.QueryRow("SELECT id FROM "+userContactGroupTableName+" WHERE name=$1", groupName).Scan(&contactGroupId)
			if gErr != nil {
				giErr := r.db.QueryRow("INSERT INTO "+userContactGroupTableName+" (name) VALUES ($1) RETURNING id", groupName).Scan(&contactGroupId)
				if giErr != nil {
					log.Printf("InsertGroupError: %+v", giErr)
				}
			}
			//log.Printf("Group: %s, GroupId: %d", groupName, contactGroupId)
			if contactGroupId > 0 {
				_, _ = r.db.Exec("INSERT INTO "+userContactGroupContactTableName+" (contact_id, group_id) VALUES ($1, $2)", contact.Id, contactGroupId)
			}
		}
	}

	return err
}

func (r *Repo) GetUserContactById(contactId int, userId int) (contact entity.UserContact, err error) {
	var query *sql.Row
	var q = "SELECT id,user_id,name,phone,in_wa,verified_name FROM " + userContactTableName + " WHERE id=$1"
	if userId != 0 {
		query = r.db.QueryRow(q+" AND user_id=$2", contactId, userId)
	} else {
		query = r.db.QueryRow(q)
	}

	var (
		id, in_wa                  int
		user_id                    sql.NullInt64
		name, phone, verified_name sql.NullString
	)
	err = query.Scan(
		&id,
		&user_id,
		&name,
		&phone,
		&in_wa,
		&verified_name,
	)

	contact = entity.UserContact{
		Id:           id,
		UserId:       int(user_id.Int64),
		Name:         name.String,
		Phone:        phone.String,
		InWA:         in_wa,
		VerifiedName: verified_name.String,
	}

	return contact, err
}

func (r *Repo) DeleteUserContactById(contactId int, userId int) error {
	_, err := r.db.Exec("DELETE FROM "+userContactTableName+" WHERE id=$1 AND user_id=$2", contactId, userId)

	return err
}

func (r *Repo) GetRandomContactNotInWA() (contact entity.UserContact, err error) {
	var (
		id, in_wa                  int
		user_id                    sql.NullInt64
		name, phone, verified_name sql.NullString
	)

	err = r.db.QueryRow("SELECT id,user_id,name,phone,in_wa,verified_name FROM "+userContactTableName+" WHERE in_wa=0 ORDER BY RANDOM() LIMIT 1").Scan(
		&id,
		&user_id,
		&name,
		&phone,
		&in_wa,
		&verified_name,
	)
	contact = entity.UserContact{
		Id:           id,
		UserId:       int(user_id.Int64),
		Name:         name.String,
		Phone:        phone.String,
		InWA:         in_wa,
		VerifiedName: verified_name.String,
	}
	return contact, err
}

func contactFilterField(f string) string {
	fields := map[string]string{"p": "phone", "n": "name", "g": "group"}
	field, isExist := fields[f]
	if isExist {
		return field
	}

	return ""
}
