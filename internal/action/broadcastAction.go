package action

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

func (a *Action) ActionPostBroadcastMessage(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)
	totalRecipient := 0

	responsePayload.Status = false

	reqBody := new(entity.Broadcast)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		return err
	}

	uDevice := c.Get("device").(*entity.Device)

	switch reqBody.ContactType {
	case "c":
		totalRecipient, _ = a.service.Repo.GetTotalUserContacts(a.user.UserId, reqBody.ContactFilter, reqBody.FilterValue)
	case "w":
		totalRecipient, _ = a.service.Repo.CountWhatsAppContact(uDevice, reqBody.ContactFilter, reqBody.FilterValue)
	case "p":
		if reqBody.Phones != nil {
			totalRecipient = len(reqBody.Phones)
		}
	}

	if totalRecipient == 0 {
		responsePayload.Message = "No recipients, please change recipient filter"

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	reqBody.UserId = a.user.UserId
	reqBody.Jid = uDevice.Jid.ToNonAD()

	err = a.service.Repo.SaveBroadcast(reqBody)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Message = "Broadcast has been successfully saved"
	responsePayload.Status = true

	return c.JSON(http.StatusCreated, responsePayload)
}

type broadcastsResponsePayload struct {
	Broadcasts []*entity.Broadcast `json:"broadcasts"`
	Total      int                 `json:"total"`
	PrevPage   int                 `json:"prevPage"`
	NextPage   int                 `json:"nextPage"`
	Limit      int                 `json:"limit"`
}

func (a *Action) ActionGetBroadcasts(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		broadcasts      []*entity.Broadcast
		totalBroadcast  int
	)

	responsePayload.Status = false

	limit := 20
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}
	offset := (page - 1) * limit

	uDevice := c.Get("device").(*entity.Device)
	broadcasts, totalBroadcast, err = a.service.Repo.GetBroadcasts(a.user.UserId, uDevice.Jid.ToNonAD(), limit, offset)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	prevPage := 0
	if page > 1 {
		prevPage = page - 1
	}
	nextPage := 0
	if (limit + offset) < totalBroadcast {
		nextPage = page + 1
	}

	responsePayload.Status = true
	responsePayload.Data = broadcastsResponsePayload{
		Broadcasts: broadcasts,
		Total:      totalBroadcast,
		PrevPage:   prevPage,
		NextPage:   nextPage,
		Limit:      limit,
	}

	return c.JSON(http.StatusOK, responsePayload)
}

type toggleRunBody struct {
	Status string `json:"status"`
}

func (a *Action) ActionPatchToggleRun(c echo.Context) error {
	var (
		err             error
		body            toggleRunBody
		responsePayload ResponsePayload
		broadcastId     int
	)

	code := http.StatusOK
	responsePayload.Status = true

	broadcastId, _ = strconv.Atoi(c.Param("broadcastId"))

	err = c.Bind(&body)
	if err == nil {
		err = a.service.Repo.UpdateRunningStatusBroadcast(int64(broadcastId), body.Status)
	}

	if err != nil {
		code = http.StatusUnprocessableEntity
		responsePayload.Status = false
		responsePayload.Message = err.Error()
	} else {
		if body.Status == "start" {
			responsePayload.Data = "pending"
		} else {
			responsePayload.Data = "pause"
		}
	}

	return c.JSON(code, responsePayload)
}

func (a *Action) ActionDeleteBroadcast(c echo.Context) error {
	var (
		err             error
		broadcastId     int
		responsePayload ResponsePayload
	)

	code := http.StatusOK
	responsePayload.Status = true

	broadcastId, err = strconv.Atoi(c.Param("broadcastId"))
	if err == nil {
		uDevice := c.Get("device").(*entity.Device)
		err = a.service.Repo.DeleteBroadcast(broadcastId, uDevice.Jid)
	}

	if err != nil {
		code = http.StatusUnprocessableEntity
		responsePayload.Status = false
		responsePayload.Message = err.Error()
	}

	return c.JSON(code, responsePayload)
}

type broadcastRecipientsResponsePayload struct {
	Recipients []entity.BroadcastRecipient `json:"recipients"`
	Total      int                         `json:"total"`
	PrevPage   int                         `json:"prevPage"`
	NextPage   int                         `json:"nextPage"`
	Limit      int                         `json:"limit"`
}

func (a *Action) ActionGetBroadCastRecipients(c echo.Context) error {
	var (
		err             error
		broadcastId     int
		total           int
		recipients      []entity.BroadcastRecipient
		responsePayload ResponsePayload
	)

	code := http.StatusOK
	responsePayload.Status = true

	limit := 50
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}
	offset := (page - 1) * limit

	broadcastId, err = strconv.Atoi(c.Param("broadcastId"))
	if err == nil {
		recipients, total, err = a.service.Repo.GetBroadcastRecipients(int64(broadcastId), limit, offset)
		if err == nil {
			prevPage := 0
			if page > 1 {
				prevPage = page - 1
			}
			nextPage := 0
			if (limit + offset) < total {
				nextPage = page + 1
			}

			responsePayload.Data = broadcastRecipientsResponsePayload{
				Recipients: recipients,
				Total:      total,
				PrevPage:   prevPage,
				NextPage:   nextPage,
				Limit:      limit,
			}
		}
	}

	if err != nil {
		code = http.StatusUnprocessableEntity
		responsePayload.Status = false
		responsePayload.Message = err.Error()
	}

	return c.JSON(code, responsePayload)
}
