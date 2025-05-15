package action

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

type updateReqPayload struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password"`
}

func (a *Action) actionPostUpdateAccount(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)
	responsePayload.Status = false

	account := new(updateReqPayload)
	if err = c.Bind(account); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(account); err != nil {
		return err
	}

	user := entity.User{
		Id: a.user.UserId,
		Name: account.Name,
		Email: account.Email,
		Password: account.Password,
	}

	err = a.service.Repo.SaveUser(&user)
	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	responsePayload.Status = true

	return c.JSON(http.StatusOK, responsePayload)
}
