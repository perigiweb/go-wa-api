package action

import (
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

type waQRResponsePayload struct {
	QRImage   string         `json:"qrImage"`
	QRTimeout int            `json:"qrTimeout"`
	Device    *entity.Device `json:"device"`
}

func (a *Action) ActionPostWhatsAppQR(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		qrImage         string
		qrTimeout       int
	)

	log.Println("ActionPostWhatsAppQR")

	responsePayload.Status = false

	uDevice := c.Get("device").(*entity.Device)

	qrImage, qrTimeout, err = a.service.WhatsAppLogin(uDevice)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Message = "Generate QR"
	responsePayload.Data = waQRResponsePayload{
		QRImage:   qrImage,
		QRTimeout: qrTimeout,
		Device:    uDevice,
	}

	return c.JSON(http.StatusOK, responsePayload)
}

type waSendMsgPayload struct {
	Recipient    string              `json:"recipient" validate:"required"`
	Message      string              `json:"message" validate:"required"`
	MessageType  string              `json:"mType"`
	UploadedFile entity.UploadedFile `json:"uploadedFile"`
}

func (a *Action) ActionPostSendMessage(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		sendResponse    whatsmeow.SendResponse
	)

	responsePayload.Status = false

	reqBody := new(waSendMsgPayload)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		//responsePayload.Message = err.Error()
		//return c.JSON(http.StatusOK, responsePayload)
		return err
	}

	uDevice := c.Get("device").(*entity.Device)

	sendResponse, err = a.service.SendMessage(uDevice.Id, reqBody.Recipient, reqBody.Message, reqBody.UploadedFile)

	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = sendResponse

	return c.JSON(http.StatusOK, responsePayload)
}

type chatPresenceReqPayload struct {
	Phone string                  `json:"phone" validate:"required"`
	State types.ChatPresence      `json:"state" validate:"required"`
	Media types.ChatPresenceMedia `json:"media"`
}

func (a *Action) ActionPostSendChatPresence(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)

	responsePayload.Status = false
	reqBody := new(chatPresenceReqPayload)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		return err
	}

	uDevice := c.Get("device").(*entity.Device)
	err = a.service.SendChatPresence(uDevice.Id, reqBody.Phone, reqBody.State, reqBody.Media)
	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true

	return c.JSON(http.StatusOK, responsePayload)
}

type waCheckPhoneReqPayload struct {
	Phones []string `json:"phones" validate:"required"`
}

func (a *Action) ActionPostCheckPhone(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		onWhatsApp      []types.IsOnWhatsAppResponse
	)

	responsePayload.Status = false

	reqBody := new(waCheckPhoneReqPayload)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		return err
	}

	uDevice := c.Get("device").(*entity.Device)

	onWhatsApp, err = a.service.CheckPhone(uDevice.Id, reqBody.Phones)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = onWhatsApp

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionGetProfilePicture(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		profilePicture  *types.ProfilePictureInfo
		jid             string
	)

	responsePayload.Status = false

	uDevice := c.Get("device").(*entity.Device)

	jid = c.QueryParam("jid")
	if jid == "" {
		jid = uDevice.Jid.String()
	}

	log.Printf("JID: %s", jid)
	//pjid, _ := types.ParseJID(jid)
	//log.Printf("ParsedJID: %+v", pjid)

	profilePicture, err = a.service.GetProfilePicture(uDevice.Id, jid, "")
	if err != nil {
		log.Printf("Error: %+v", err)
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = profilePicture

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionGetProfileInfo(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		profilePicture  map[types.JID]types.UserInfo //*types.ProfilePictureInfo
		jid             string
	)

	responsePayload.Status = false

	uDevice := c.Get("device").(*entity.Device)

	jid = c.QueryParam("jid")
	if jid == "" {
		jid = uDevice.Jid.String()
	}

	log.Printf("JID: %s", jid)
	pjid, _ := types.ParseJID(jid)
	log.Printf("ParsedJID: %+v", pjid)

	profilePicture, err = a.service.GetProfileInfo(uDevice.Id, pjid, "")
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = profilePicture

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionGetWhatsAppContacts(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		allContacts     map[types.JID]types.ContactInfo
	)

	responsePayload.Status = false

	uDevice := c.Get("device").(*entity.Device)

	allContacts, err = a.service.GetAllWhatsAppContacts(uDevice.Id)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = allContacts

	return c.JSON(http.StatusOK, responsePayload)
}

type markAsReadReqPayload struct {
	MessageIds []types.MessageID `json:"messageIds" validate:"required"`
	Chat       types.JID         `json:"chat" validate:"required"`
}

func (a *Action) ActionPostMarkAsRead(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)

	responsePayload.Status = false
	reqBody := new(markAsReadReqPayload)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	uDevice := c.Get("device").(*entity.Device)
	err = a.service.MarkAsRead(uDevice.Id, reqBody.Chat, reqBody.MessageIds)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionGetChats(c echo.Context) error {
	var (
		responsePayload ResponsePayload
	)

	responsePayload.Status = false
	uDevice := c.Get("device").(*entity.Device)
	chats, err := a.service.Repo.GetWAChats(uDevice.Id)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = chats

	return c.JSON(http.StatusOK, responsePayload)
}

type conversationParam struct {
	ChatId types.JID `query:"c" validate:"required"`
}

func (a *Action) ActionGetConversation(c echo.Context) error {
	var (
		err             error
		messages        []entity.UserMessage
		responsePayload ResponsePayload
	)

	responsePayload.Status = false
	reqQuery := new(conversationParam)
	if err = c.Bind(reqQuery); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(reqQuery); err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	uDevice := c.Get("device").(*entity.Device)

	messages, err = a.service.Repo.GetWaConversation(uDevice.Id, reqQuery.ChatId, time.Time{})
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = messages

	return c.JSON(http.StatusOK, responsePayload)
}
