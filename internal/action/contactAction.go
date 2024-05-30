package action

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

func (a *Action) ActionGetTotalUserContacts(c echo.Context) error {
	var responsePayload ResponsePayload
	responsePayload.Status = false

	totalContact, err := a.service.Repo.GetTotalUserContacts(a.user.UserId, "", "")

	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = totalContact

	return c.JSON(http.StatusOK, responsePayload)
}

type contactsResponsePayload struct {
	Contacts []entity.UserContact `json:"contacts"`
	Total    int                  `json:"total"`
	PrevPage int                  `json:"prevPage"`
	NextPage int                  `json:"nextPage"`
	Limit    int                  `json:"limit"`
}

func (a *Action) ActionGetUserContacts(c echo.Context) error {
	var responsePayload ResponsePayload
	responsePayload.Status = false

	limit := 100
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 1
	}
	offset := (page - 1) * limit

	s := c.QueryParam("q")

	contacts, totalContact, err := a.service.Repo.GetUserContacts(a.user.UserId, limit, offset, s)
	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusOK, responsePayload)
	}

	prevPage := 0
	if page > 1 {
		prevPage = page - 1
	}
	nextPage := 0
	if (limit + offset) < totalContact {
		nextPage = page + 1
	}

	responsePayload.Status = true
	responsePayload.Data = &contactsResponsePayload{
		Contacts: contacts,
		Total:    totalContact,
		PrevPage: prevPage,
		NextPage: nextPage,
		Limit:    limit,
	}

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionPostUserContact(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)

	responsePayload.Status = false

	contact := new(entity.UserContact)
	if err = c.Bind(contact); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusOK, responsePayload)
	}

	if err = c.Validate(contact); err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	if contact.UserId == 0 {
		contact.UserId = a.user.UserId
	}

	if contact.UserId != a.user.UserId {
		responsePayload.Message = "You are not allowed to modify this contact"
		return c.JSON(http.StatusOK, responsePayload)
	}

	err = a.service.Repo.SaveUserContact(contact)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = contact

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionGetUserContact(c echo.Context) error {
	var responsePayload ResponsePayload
	responsePayload.Status = false

	contactId, err := strconv.Atoi(c.Param("contactId"))
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	contact, err := a.service.Repo.GetUserContactById(contactId, a.user.UserId)

	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = contact

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionDeleteUserContact(c echo.Context) (err error) {
	var (
		responsePayload ResponsePayload
		contactId       int
	)

	responsePayload.Status = false

	contactId, err = strconv.Atoi(c.Param("contactId"))
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	err = a.service.Repo.DeleteUserContactById(contactId, a.user.UserId)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true

	return c.JSON(http.StatusOK, responsePayload)
}
