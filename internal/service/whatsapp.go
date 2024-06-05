package service

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"runtime"
	"strings"

	qrCode "github.com/skip2/go-qrcode"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	waproto "go.mau.fi/whatsmeow/binary/proto"
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
			wastore.DeviceProps.RequireFullSync = proto.Bool(true)
			wastore.DeviceProps.HistorySyncConfig = &waproto.DeviceProps_HistorySyncConfig{
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
		whatsAppClients[uDevice.Id].DontSendSelfBroadcast = true

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

func (s *Service) SendMessage(deviceId string, recipient string, message string, file entity.UploadedFile) (whatsmeow.SendResponse, error) {
	if file.Data != "" {
		return s.SendImageMessage(deviceId, recipient, message, file.Data)
	}

	return s.SendTextMessage(deviceId, recipient, message)
}

func (s *Service) SendTextMessage(deviceId string, recipient string, message string) (whatsmeow.SendResponse, error) {
	var r whatsmeow.SendResponse

	c := getWAClient(deviceId)
	if c == nil {
		return r, errors.New("whatsapp client not found or not logged in")
	}

	to, err := parseJID(recipient)
	if err != nil {
		return r, err
	}

	waMsg := &waproto.Message{
		ExtendedTextMessage: &waproto.ExtendedTextMessage{
			Text: &message,
		},
	}

	return c.SendMessage(context.Background(), to, waMsg)
}

func (s *Service) SendImageMessage(deviceId string, recipient string, caption string, fileData string) (whatsmeow.SendResponse, error) {
	var (
		err      error
		r        whatsmeow.SendResponse
		uploaded whatsmeow.UploadResponse
		imgData  []byte
		dataURL  *dataurl.DataURL
	)

	c := getWAClient(deviceId)
	if c == nil {
		return r, errors.New("whatsapp client not found or not logged in")
	}

	to, err := parseJID(recipient)
	if err != nil {
		return r, err
	}

	if fileData[0:10] != "data:image" {
		return r, errors.New("image data should start with \"data:image/type;base64,\"; type can be png, jpg, jpeg")
	}

	dataURL, err = dataurl.DecodeString(fileData)
	if err != nil {
		return r, err
	}

	imgData = dataURL.Data
	uploaded, err = c.Upload(context.Background(), imgData, whatsmeow.MediaImage)
	if err != nil {
		return r, err
	}

	waMsg := &waproto.Message{
		ImageMessage: &waproto.ImageMessage{
			Caption:       proto.String(caption),
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(imgData)),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
		},
	}

	return c.SendMessage(context.Background(), to, waMsg)
}

func (s *Service) SendBroadcastMessage(broadcastToSend entity.BroadcastToSend) (whatsmeow.SendResponse, error) {
	return s.SendMessage(
		broadcastToSend.Broadcast.DeviceId,
		broadcastToSend.Recipient.Phone,
		broadcastToSend.Broadcast.Message,
		*broadcastToSend.Broadcast.Media,
	)
}

func (s *Service) GetAllWhatsAppContacts(deviceId string) (contacts map[types.JID]types.ContactInfo, err error) {
	c := getWAClient(deviceId)
	if c == nil {
		return contacts, errors.New("whatsapp client not found or not logged in")
	}

	return c.Store.Contacts.GetAllContacts()
}

func whatsAppGetUserAgent(agentType string) waproto.DeviceProps_PlatformType {
	switch strings.ToLower(agentType) {
	case "desktop":
		return waproto.DeviceProps_DESKTOP
	case "mac":
		return waproto.DeviceProps_CATALINA
	case "android":
		return waproto.DeviceProps_ANDROID_AMBIGUOUS
	case "android-phone":
		return waproto.DeviceProps_ANDROID_PHONE
	case "andorid-tablet":
		return waproto.DeviceProps_ANDROID_TABLET
	case "ios-phone":
		return waproto.DeviceProps_IOS_PHONE
	case "ios-catalyst":
		return waproto.DeviceProps_IOS_CATALYST
	case "ipad":
		return waproto.DeviceProps_IPAD
	case "wearos":
		return waproto.DeviceProps_WEAR_OS
	case "ie":
		return waproto.DeviceProps_IE
	case "edge":
		return waproto.DeviceProps_EDGE
	case "chrome":
		return waproto.DeviceProps_CHROME
	case "safari":
		return waproto.DeviceProps_SAFARI
	case "firefox":
		return waproto.DeviceProps_FIREFOX
	case "opera":
		return waproto.DeviceProps_OPERA
	case "uwp":
		return waproto.DeviceProps_UWP
	case "aloha":
		return waproto.DeviceProps_ALOHA
	case "tv-tcl":
		return waproto.DeviceProps_TCL_TV
	default:
		return waproto.DeviceProps_UNKNOWN
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
