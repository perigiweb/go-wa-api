package action

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/vincent-petithory/dataurl"

	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

func (a *Action) ActionGetTotalUserContacts(c echo.Context) error {
	var responsePayload ResponsePayload
	responsePayload.Status = false

	f := c.QueryParam("f") // filter by
	v := c.QueryParam("v") // filter value
	totalContact, err := a.service.Repo.GetTotalUserContacts(a.user.UserId, f, v)

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

		return c.JSON(http.StatusBadRequest, responsePayload)
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

func (a *Action) ActionGetUserContactGroups(c echo.Context) error {
	var responsePayload ResponsePayload

	contactGroups, _ := a.service.Repo.GetUserContactGroups(a.user.UserId)

	responsePayload.Status = true
	responsePayload.Data = contactGroups

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

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(contact); err != nil {
		return err
	}

	if contact.UserId == 0 {
		contact.UserId = a.user.UserId
	}

	if contact.UserId != a.user.UserId {
		responsePayload.Message = "You are not allowed to modify this contact"
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	err = a.service.Repo.SaveUserContact(contact)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = contact

	return c.JSON(http.StatusCreated, responsePayload)
}

type importContactPayload struct {
	UploadedFile string `json:"UploadedFile" validate:"required"`
}
type importContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Group string `json:"group"`
}

func (a *Action) ActionPostImportContact(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		importContacts  []importContact
	)

	responsePayload.Status = false
	importPayload := new(importContactPayload)
	if err = c.Bind(importPayload); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(importPayload); err != nil {
		return err
	}

	dataURL, err := dataurl.DecodeString(importPayload.UploadedFile)
	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	err = json.Unmarshal(dataURL.Data, &importContacts)
	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	for _, ic := range importContacts {
		importedContact := entity.UserContact{
			UserId: a.user.UserId,
			Name:   ic.Name,
			Phone:  ic.Phone,
			InWA:   1,
		}

		if ic.Group != "" {
			groups := strings.Split(ic.Group, ",")
			for _, g := range groups {
				importedContact.Groups = append(importedContact.Groups, strings.TrimSpace(g))
			}
		}

		err = a.service.Repo.SaveUserContact(&importedContact)
		if err != nil {
			responsePayload.Message = err.Error()

			return c.JSON(http.StatusUnprocessableEntity, responsePayload)
		}
	}

	responsePayload.Status = true

	return c.JSON(http.StatusCreated, responsePayload)
}

func (a *Action) ActionGetUserContact(c echo.Context) error {
	var responsePayload ResponsePayload
	responsePayload.Status = false

	contactId, err := strconv.Atoi(c.Param("contactId"))
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	contact, err := a.service.Repo.GetUserContactById(contactId, a.user.UserId)

	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
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
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	err = a.service.Repo.DeleteUserContactById(contactId, a.user.UserId)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true

	return c.JSON(http.StatusOK, responsePayload)
}
