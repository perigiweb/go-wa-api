package action

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
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
	)

	responsePayload.Status = false

	reqBody := new(waSendMsgPayload)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusOK, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		//responsePayload.Message = err.Error()
		//return c.JSON(http.StatusOK, responsePayload)
		return err
	}

	uDevice := c.Get("device").(*entity.Device)

	if reqBody.MessageType == "media" {
		resp, err := a.service.SendImageMessage(uDevice.Id, reqBody.Recipient, reqBody.Message, reqBody.UploadedFile.Data)

		if err != nil {
			responsePayload.Message = err.Error()
			return c.JSON(http.StatusOK, responsePayload)
		}

		responsePayload.Status = true
		responsePayload.Data = resp

		return c.JSON(http.StatusOK, responsePayload)
	} else {
		resp, err := a.service.SendTextMessage(uDevice.Id, reqBody.Recipient, reqBody.Message)

		if err != nil {
			responsePayload.Message = err.Error()
			return c.JSON(http.StatusOK, responsePayload)
		}

		responsePayload.Status = true
		responsePayload.Data = resp

		return c.JSON(http.StatusOK, responsePayload)
	}
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

		return c.JSON(http.StatusOK, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	uDevice := c.Get("device").(*entity.Device)

	onWhatsApp, err = a.service.CheckPhone(uDevice.Id, reqBody.Phones)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
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
		return c.JSON(http.StatusOK, responsePayload)
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
		return c.JSON(http.StatusOK, responsePayload)
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
		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = allContacts

	return c.JSON(http.StatusOK, responsePayload)
}
