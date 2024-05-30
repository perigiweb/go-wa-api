package action

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

func (a *Action) ActionGetDevices(c echo.Context) error {
	var responsePayload ResponsePayload
	responsePayload.Status = false

	devices, err := a.service.Repo.GetDevicesByUserId(a.user.UserId)
	if err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = devices

	return c.JSON(http.StatusOK, responsePayload)
}

type AddDeviceReqPayload struct {
	Name string `json:"name" validate:"required"`
}

func (a *Action) ActionPostAddDevice(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
		device          entity.Device
	)

	responsePayload.Status = false

	reqBody := new(AddDeviceReqPayload)
	if err = c.Bind(reqBody); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusOK, responsePayload)
	}

	if err = c.Validate(reqBody); err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	device, err = a.service.Repo.InsertNewDevice(a.user.UserId, reqBody.Name)
	if err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusOK, responsePayload)
	}

	responsePayload.Status = true
	responsePayload.Data = device

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionDeleteDevice(c echo.Context) error {
	var (
		//err error
		responsePayload ResponsePayload
	)

	err := a.service.Repo.DeleteDeviceById(c.Param("deviceId"), a.user.UserId)

	responsePayload.Status = true
	if err != nil {
		responsePayload.Status = false
		responsePayload.Message = err.Error()
	}

	return c.JSON(http.StatusOK, responsePayload)
}
