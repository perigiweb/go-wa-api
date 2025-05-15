package action

import (
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type (
	signUpRequestPayload struct {
		Name     string `json:"name" validate:"required"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}

	signInRequestPayload struct {
		UserEmail    string `json:"userEmail" validate:"required,email"`
		UserPassword string `json:"userPassword" validate:"required"`
	}

	authData struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}

	signOutRequestPayload struct {
		SessionId string `json:"sessionId"`
	}
)

func (a *Action) ActionPostSignUp(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)
	responsePayload.Status = false

	u := new(signUpRequestPayload)
	if err = c.Bind(u); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(u); err != nil {
		return err
	}

	responsePayload.Message = "Sorry, Sign Up not open yet :-)"
	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionSignIn(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)
	responsePayload.Status = false

	u := new(signInRequestPayload)
	if err = c.Bind(u); err != nil {
		responsePayload.Message = err.Error()

		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if err = c.Validate(u); err != nil {
		return err
	}

	status, message, jwtToken, jwtRefreshToken := a.service.SignIn(u.UserEmail, u.UserPassword, c.Request().UserAgent(), c.Request().RemoteAddr)

	responsePayload.Status = status
	responsePayload.Message = message
	responsePayload.Data = &authData{
		Token:        jwtToken,
		RefreshToken: jwtRefreshToken,
	}

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionRefreshToken(c echo.Context) error {
	var responsePayload ResponsePayload

	responsePayload.Status = false

	authRefreshToken := c.Get("refreshToken").(*jwt.Token)

	log.Println("ActionRefreshToken")

	status, message, jwtToken, jwtRefreshToken := a.service.SignInWithRefreshToken(authRefreshToken)

	responsePayload.Status = status
	responsePayload.Message = message
	responsePayload.Data = &authData{
		Token:        jwtToken,
		RefreshToken: jwtRefreshToken,
	}

	return c.JSON(http.StatusOK, responsePayload)
}

func (a *Action) ActionSignOut(c echo.Context) error {
	var (
		err             error
		responsePayload ResponsePayload
	)

	responsePayload.Status = false

	p := new(signOutRequestPayload)
	if err = c.Bind(p); err != nil {
		responsePayload.Message = err.Error()
		return c.JSON(http.StatusUnprocessableEntity, responsePayload)
	}

	if p.SessionId != "" {
		err = a.service.SignOut(p.SessionId)
		if err != nil {
			responsePayload.Message = err.Error()
			return c.JSON(http.StatusUnprocessableEntity, responsePayload)
		}
	}

	responsePayload.Status = true
	return c.JSON(http.StatusOK, responsePayload)
}
