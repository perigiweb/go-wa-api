package service

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	qrCode "github.com/skip2/go-qrcode"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	wastore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"

	"github.com/vincent-petithory/dataurl"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

var whatsAppClients = make(map[string]*whatsmeow.Client)

func (s *Service) WhatsAppCreateClient(uDevice *entity.Device) error {
	log.Println("WhatsApp create client")

	var (
		err      error
		waDevice *wastore.Device
	)

	if whatsAppClients[uDevice.Id] == nil {
		log.Println("WhatsApp Client Not found, create new client")
		if uDevice.Jid == nil {
			waDevice = s.waDataStore.NewDevice()

			wastore.DeviceProps.Os = proto.String(whatsAppGetUserOS())
			wastore.DeviceProps.PlatformType = whatsAppGetUserAgent("chrome").Enum()
			wastore.DeviceProps.RequireFullSync = proto.Bool(false)
			wastore.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
				FullSyncDaysLimit:   proto.Uint32(1),
				FullSyncSizeMbLimit: proto.Uint32(10),
				StorageQuotaMb:      proto.Uint32(10),
			}
		} else {
			waDevice, err = s.waDataStore.GetDevice(*uDevice.Jid)
			if err != nil {
				return err
			}
		}

		whatsAppClients[uDevice.Id] = whatsmeow.NewClient(waDevice, nil)
		whatsAppClients[uDevice.Id].EnableAutoReconnect = true
		whatsAppClients[uDevice.Id].AutoTrustIdentity = true

		registerWAEventHandler(whatsAppClients[uDevice.Id], s.Repo, uDevice)
	}

	return nil
}

func (s *Service) WhatsAppLogin(uDevice *entity.Device) (string, int, error) {
	var err error

	log.Println("WhatsApp Login...")
	err = s.WhatsAppCreateClient(uDevice)
	if err != nil {
		return "", 0, err
	}

	log.Println(whatsAppClients[uDevice.Id].Store.ID)
	if whatsAppClients[uDevice.Id].IsLoggedIn() {
		log.Println("LoggedIn")
		return "LoggedIn", 0, nil
	}

	//if whatsAppClients[uDevice.Id].IsConnected() {
	//	return "ConnectedButNotLoggedIn", 0, nil
	//}

	if whatsAppClients[uDevice.Id].Store.ID == nil {
		log.Println("Generate QR Image")

		qrChanGenerate, _ := whatsAppClients[uDevice.Id].GetQRChannel(context.Background())

		// Connect WebSocket while Initialize QR Code Data to be Sent
		if !whatsAppClients[uDevice.Id].IsConnected() {
			err = whatsAppClients[uDevice.Id].Connect()
			if err != nil {
				return "", 0, err
			}
		}

		// Get Generated QR Code and Timeout Information
		qrImage, qrTimeout := whatsAppGenerateQR(qrChanGenerate)

		return "data:image/png;base64," + qrImage, qrTimeout, nil
	} else {
		log.Println("WhatsAppClinet.Store.ID not nil, Reconnected")
		err = s.WhatsAppReconnect(uDevice)
		if err != nil {
			return "", 0, err
		}

		return "Reconnected", 30, nil
	}
}

func (s *Service) WhatsAppReconnect(uDevice *entity.Device) error {
	var err error

	err = s.WhatsAppCreateClient(uDevice)
	if err != nil {
		return err
	}

	whatsAppClients[uDevice.Id].Disconnect()
	if whatsAppClients[uDevice.Id] != nil {
		err = whatsAppClients[uDevice.Id].Connect()
		return err
	}

	return errors.New("cannot re-connect. please re-login and scan QR Code again")
}

func (s *Service) GetProfilePicture(userDeviceId string, jid string, existingId string) (*types.ProfilePictureInfo, error) {

	parsedJID, err := parseJID(jid)
	if err != nil {
		return nil, err
	}

	log.Printf("ParsedJID: %v", parsedJID)

	waClient := getWAClient(userDeviceId)
	if waClient == nil {
		return nil, errors.New("whatsapp client for id: " + userDeviceId + " not found or not logged in")
	}

	params := &whatsmeow.GetProfilePictureParams{
		Preview:     true,
		ExistingID:  existingId,
		IsCommunity: false,
	}

	return waClient.GetProfilePictureInfo(parsedJID, params)
}

func (s *Service) GetProfileInfo(userDeviceId string, jid types.JID, existingId string) (map[types.JID]types.UserInfo, error) {
	/*
		j, err := parseJID(jid)
		if err != nil {
			return nil, err
		}
	*/

	waClient := getWAClient(userDeviceId)
	if waClient == nil {
		return make(map[types.JID]types.UserInfo), errors.New("whatsapp client for id: " + userDeviceId + " not found or not logged in")
	}

	var jids []types.JID

	jids = append(jids, jid)

	return waClient.GetUserInfo(jids)
}

func (s *Service) CheckPhone(deviceId string, phones []string) ([]types.IsOnWhatsAppResponse, error) {
	c := getWAClient(deviceId)
	if c == nil {
		return nil, errors.New("whatsapp client not found or not logged in")
	}

	return c.IsOnWhatsApp(phones)
}

func (s *Service) SendMessage(deviceId string, recipient string, message string, file entity.UploadedFile) (r whatsmeow.SendResponse, err error) {
	var (
		to      types.JID
		waMsg   *waE2E.Message
		msgType string
	)

	c := getWAClient(deviceId)
	if c == nil {
		err = errors.New("whatsapp client not found or not logged in")
		return
	}

	to, err = parseJID(recipient)
	log.Printf("Sent To: %s", to.String())
	if err != nil {
		return
	}

	if file.Data != "" {
		var (
			uploaded whatsmeow.UploadResponse
			imgData  []byte
			dataURL  *dataurl.DataURL
		)
		if file.Data[0:10] != "data:image" {
			err = errors.New("image data should start with \"data:image/type;base64,\"; type can be png, jpg, jpeg")
			return
		}

		dataURL, err = dataurl.DecodeString(file.Data)
		if err != nil {
			return
		}

		imgData = dataURL.Data
		uploaded, err = c.Upload(context.Background(), imgData, whatsmeow.MediaImage)
		if err != nil {
			return
		}

		waMsg = &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption:       proto.String(message),
				Mimetype:      proto.String(http.DetectContentType(imgData)),
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
			},
		}
		msgType = "media"
	} else {
		waMsg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: &message,
			},
		}
		msgType = "text"
	}

	r, err = c.SendMessage(context.Background(), to, waMsg)

	if err == nil {
		waMessage := &entity.WAMessage{
			Message: waMsg,
		}
		m := entity.UserMessage{
			ID:        r.ID,
			DeviceId:  deviceId,
			TheirJID:  &to,
			Message:   waMessage,
			FromMe:    true,
			Timestamp: r.Timestamp,
			Type:      msgType,
		}

		s.Repo.InsertWAMessage(m)
	}

	return
}

func (s *Service) SendBroadcastMessage(broadcastToSend *entity.BroadcastToSend) (*whatsmeow.SendResponse, error) {
	if broadcastToSend == nil {
		return nil, nil
	}
	/*
		max := 15
		min := 5
		x := rand.Intn(max-min) + min

		s.SendChatPresence(broadcastToSend.Broadcast.DeviceId, broadcastToSend.Recipient.Phone, "composing", "")

		time.Sleep(time.Duration(x) * time.Second)
	*/
	response, err := s.SendMessage(
		broadcastToSend.Broadcast.Device.Id,
		broadcastToSend.Recipient.Phone,
		broadcastToSend.Broadcast.Message,
		*broadcastToSend.Broadcast.Media,
	)

	return &response, err
}

func (s *Service) MarkAsRead(deviceId string, chat types.JID, messageIds []types.MessageID) error {
	c := getWAClient(deviceId)
	if c == nil {
		return errors.New("whatsapp client not found or not logged in")
	}

	return c.MarkRead(messageIds, time.Now(), chat, *c.Store.ID)
}

func (s *Service) SendChatPresence(deviceId string, phone string, state types.ChatPresence, media types.ChatPresenceMedia) error {
	c := getWAClient(deviceId)
	if c == nil {
		return errors.New("whatsapp client not found or not logged in")
	}

	jid, err := parseJID(phone)
	if err != nil {
		return err
	}

	return c.SendChatPresence(jid, state, media)
}

func (s *Service) GetAllWhatsAppContacts(deviceId string) (contacts map[types.JID]types.ContactInfo, err error) {
	c := getWAClient(deviceId)
	if c == nil {
		return contacts, errors.New("whatsapp client not found or not logged in")
	}

	return c.Store.Contacts.GetAllContacts()
}

func whatsAppGetUserAgent(agentType string) waCompanionReg.DeviceProps_PlatformType {
	switch strings.ToLower(agentType) {
	case "desktop":
		return waCompanionReg.DeviceProps_DESKTOP
	case "mac":
		return waCompanionReg.DeviceProps_CATALINA
	case "android":
		return waCompanionReg.DeviceProps_ANDROID_AMBIGUOUS
	case "android-phone":
		return waCompanionReg.DeviceProps_ANDROID_PHONE
	case "andorid-tablet":
		return waCompanionReg.DeviceProps_ANDROID_TABLET
	case "ios-phone":
		return waCompanionReg.DeviceProps_IOS_PHONE
	case "ios-catalyst":
		return waCompanionReg.DeviceProps_IOS_CATALYST
	case "ipad":
		return waCompanionReg.DeviceProps_IPAD
	case "wearos":
		return waCompanionReg.DeviceProps_WEAR_OS
	case "ie":
		return waCompanionReg.DeviceProps_IE
	case "edge":
		return waCompanionReg.DeviceProps_EDGE
	case "chrome":
		return waCompanionReg.DeviceProps_CHROME
	case "safari":
		return waCompanionReg.DeviceProps_SAFARI
	case "firefox":
		return waCompanionReg.DeviceProps_FIREFOX
	case "opera":
		return waCompanionReg.DeviceProps_OPERA
	case "uwp":
		return waCompanionReg.DeviceProps_UWP
	case "aloha":
		return waCompanionReg.DeviceProps_ALOHA
	case "tv-tcl":
		return waCompanionReg.DeviceProps_TCL_TV
	default:
		return waCompanionReg.DeviceProps_UNKNOWN
	}
}

func whatsAppGetUserOS() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	default:
		return "Linux"
	}
}

func whatsAppGenerateQR(qrChan <-chan whatsmeow.QRChannelItem) (string, int) {
	qrChanCode := make(chan string)
	qrChanTimeout := make(chan int)

	// Get QR Code Data and Timeout
	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				qrChanCode <- evt.Code
				qrChanTimeout <- int(evt.Timeout.Seconds())
			}
		}
	}()

	// Generate QR Code Data to PNG Image
	qrTemp := <-qrChanCode
	qrPNG, _ := qrCode.Encode(qrTemp, qrCode.Medium, 256)

	// Return QR Code PNG in Base64 Format and Timeout Information
	return base64.StdEncoding.EncodeToString(qrPNG), <-qrChanTimeout
}

func getWAClient(userDeviceId string) *whatsmeow.Client {
	if whatsAppClients[userDeviceId] != nil {
		if whatsAppClients[userDeviceId].IsLoggedIn() {
			return whatsAppClients[userDeviceId]
		}
	}

	return nil
}

func parseJID(arg string) (types.JID, error) {
	if arg == "" {
		return types.NewJID("", types.DefaultUserServer), nil
	}
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if arg[0] == '0' {
		arg = "62" + arg[1:]
	}

	// Basic only digit check for recipient phone number, we want to remove @server and .session
	phonenumber := ""
	phonenumber = strings.Split(arg, "@")[0]
	phonenumber = strings.Split(phonenumber, ":")[0]
	phonenumber = strings.Split(phonenumber, ".")[0]
	b := true
	for _, c := range phonenumber {
		if c < '0' || c > '9' {
			b = false
			break
		}
	}
	if !b {
		log.Println("Bad jid format, return empty")
		recipient, _ := types.ParseJID("")
		return recipient, nil
	}

	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), nil
	} else {
		recipient, err := types.ParseJID(arg)
		if err != nil {
			log.Println(err.Error())
			return recipient, err
		} else if recipient.User == "" {

			log.Println("Invalid jid. No server specified")
			return recipient, errors.New("invalid jid. no server specified")
		}
		return recipient, nil
	}
}
