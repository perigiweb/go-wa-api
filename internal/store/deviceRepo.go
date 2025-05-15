package store

import (
	"regexp"
	"slices"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
	meowTypes "go.mau.fi/whatsmeow/types"
)

const userDeviceTableName = "user_devices"

func (r *Repo) GetConnectedDevices() ([]entity.Device, error) {
	devices := make([]entity.Device, 0)

	rows, err := r.db.Query("SELECT id,user_id,name,jid,connected FROM " + userDeviceTableName + " WHERE jid IS NOT NULL ORDER BY name ASC")
	if err != nil {
		return devices, err
	}
	defer rows.Close()

	for rows.Next() {
		var device entity.Device
		if err := rows.Scan(&device.Id, &device.UserId, &device.Name, &device.Jid, &device.Connected); err == nil {
			devices = append(devices, device)
		}
	}

	return devices, nil
}

func (r *Repo) GetDevicesByUserId(userId int) ([]entity.Device, error) {
	devices := make([]entity.Device, 0)

	rows, err := r.db.Query("SELECT id,user_id,name,jid,connected FROM "+userDeviceTableName+" WHERE user_id=$1 ORDER BY name ASC", userId)
	if err != nil {
		return devices, err
	}
	defer rows.Close()

	for rows.Next() {
		var device entity.Device
		if err := rows.Scan(&device.Id, &device.UserId, &device.Name, &device.Jid, &device.Connected); err == nil {
			devices = append(devices, device)
		}
	}

	return devices, nil
}

func (r *Repo) GetDeviceByIdAndUserId(device *entity.Device) (*entity.Device, error) {
	var (
		q = "SELECT id,user_id,name,jid,connected FROM " + userDeviceTableName + " WHERE id=$1 AND user_id=$2"
	)
	err := r.db.QueryRow(q, device.Id, device.UserId).Scan(
		&device.Id,
		&device.UserId,
		&device.Name,
		&device.Jid,
		&device.Connected,
	)

	return device, err
}

func (r *Repo) GetDeviceByJid(jid meowTypes.JID) (*entity.Device, error) {
	var device entity.Device

	//jid.User

	err := r.db.QueryRow("SELECT id,user_id,name,jid,connected FROM "+userDeviceTableName+" WHERE jid LIKE $1", jid.User+"%").Scan(
		&device.Id,
		&device.UserId,
		&device.Name,
		&device.Jid,
		&device.Connected,
	)

	return &device, err
}

func (r *Repo) InsertNewDevice(userId int, deviceName string) (entity.Device, error) {
	var (
		err    error
		device entity.Device
	)

	device.UserId = userId
	device.Name = deviceName

	const query = "INSERT INTO " + userDeviceTableName + " (user_id, name) VALUES ($1, $2) RETURNING id"
	err = r.db.QueryRow(query, device.UserId, device.Name).Scan(&device.Id)

	if err != nil {
		return device, err
	}

	return device, nil
}

func (r *Repo) UpdateJID(jid meowTypes.JID, deviceId string) error {
	var q = "UPDATE " + userDeviceTableName + " SET jid=$1, connected=$2 WHERE id=$3"

	_, err := r.db.Exec(q, jid.String(), true, deviceId)

	return err
}

func (r *Repo) UpdateConnected(connected bool, deviceId string) error {
	var q = "UPDATE " + userDeviceTableName + " SET connected=$1 WHERE id=$2"

	_, err := r.db.Exec(q, connected, deviceId)

	return err
}

func (r *Repo) DeleteDeviceById(deviceId string, userId int) error {
	var err error

	if userId != 0 {
		_, err = r.db.Exec("DELETE FROM "+userDeviceTableName+" WHERE id=$1 AND user_id=$2", deviceId, userId)
	} else {
		_, err = r.db.Exec("DELETE FROM "+userDeviceTableName+" WHERE id=$1", deviceId)
	}

	return err
}

func (r *Repo) CountWhatsAppContact(device *entity.Device, f string, v string) (int, error) {
	var err error
	totalContact := 0
	fKeys := []string{"p", "n"}
	fFields := map[string]string{"p": "their_jid", "n": "full_name"}

	if slices.Contains(fKeys, f) && v != "" {
		if f == "p" {
			r := regexp.MustCompile("^0(.*)$")
			v = r.ReplaceAllString(v, "62$1")
		}
		err = r.db.QueryRow("SELECT COUNT(*) FROM whatsmeow_contacts WHERE our_jid=$1 AND "+fFields[f]+" ILIKE $2", device.Jid, v+"%").Scan(&totalContact)
	} else {
		err = r.db.QueryRow("SELECT COUNT(*) FROM whatsmeow_contacts WHERE our_jid=$1", device.Jid).Scan(&totalContact)
	}

	return totalContact, err
}
