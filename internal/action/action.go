package action

import (
	"net/http"

	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"

	"github.com/perigiweb/go-wa-api/internal"
	"github.com/perigiweb/go-wa-api/internal/service"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

type Action struct {
	baseUrl string
	service *service.Service
	user    *internal.AuthJWTClaims
}

type ResponsePayload struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func NewAction(baseUrl string, service *service.Service) *Action {
	return &Action{
		baseUrl: baseUrl,
		service: service,
	}
}

func (a *Action) ProcessAuthToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authToken := c.Get("authToken").(*jwt.Token)
		claims := authToken.Claims.(*internal.AuthJWTClaims)

		_, err := a.service.Repo.GetUserSessionById(claims.Session)
		if err != nil {
			log.Printf("UserSession Error: %s", err.Error())
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token or session has been logged out")
		}

		c.Set("user", claims)
		a.user = claims

		return next(c)
	}
}

func (a *Action) CheckDeviceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var (
			err             error
			responsePayload ResponsePayload
		)

		deviceId := c.Param("deviceId")

		uDevice := &entity.Device{
			Id:     deviceId,
			UserId: a.user.UserId,
		}
		uDevice, err = a.service.Repo.GetDeviceByIdAndUserId(uDevice)
		if err != nil {
			responsePayload.Status = false
			responsePayload.Message = "Can't find device with ID: " + deviceId
			return c.JSON(http.StatusOK, responsePayload)
		}

		c.Set("device", uDevice)

		return next(c)
	}
}

func (a *Action) Routes(e *echo.Echo) {
	authJWTSecret, _ := internal.GetEnvString("JWT_SECRET")
	authJWTConfig := echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(internal.AuthJWTClaims)
		},
		ContextKey: "authToken",
		SigningKey: []byte(authJWTSecret),
	}

	g := e.Group(a.baseUrl + "/me")

	g.Use(echojwt.WithConfig(authJWTConfig))
	g.Use(a.ProcessAuthToken)

	g.DELETE("/device/:deviceId", a.ActionDeleteDevice)
	g.POST("/device", a.ActionPostAddDevice)
	g.GET("/devices", a.ActionGetDevices)

	w := g.Group("/wa/:deviceId")
	w.Use(a.CheckDeviceMiddleware)
	w.GET("/status", a.ActionPostWhatsAppQR)
	w.POST("/check-phone", a.ActionPostCheckPhone)
	w.POST("/send", a.ActionPostSendMessage)
	w.POST("/broadcast", a.ActionPostBroadcastMessage)
	w.DELETE("/broadcast/:broadcastId", a.ActionDeleteBroadcast)
	w.GET("/avatar", a.ActionGetProfilePicture)
	w.GET("/contacts", a.ActionGetWhatsAppContacts)
	w.GET("/broadcasts", a.ActionGetBroadcasts)

	g.GET("/contacts", a.ActionGetUserContacts)
	g.GET("/total-contacts", a.ActionGetTotalUserContacts)
	g.POST("/contact", a.ActionPostUserContact)
	g.GET("/contact/:contactId", a.ActionGetUserContact)
	g.DELETE("/contact/:contactId", a.ActionDeleteUserContact)

	refreshTokenSecret, _ := internal.GetEnvString("JWT_RT_SECRET")
	refreshTokenConfig := echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(internal.RefreshTokenJWTClaims)
		},
		ContextKey: "refreshToken",
		SigningKey: []byte(refreshTokenSecret),
	}
	rt := e.Group(a.baseUrl + "/refresh-token")
	rt.Use(echojwt.WithConfig(refreshTokenConfig))
	rt.GET("", a.ActionRefreshToken)

	e.POST(a.baseUrl+"/sign-in", a.ActionSignIn)
	e.POST(a.baseUrl+"/sign-out", a.ActionSignOut)

	e.GET(a.baseUrl+"/", func(c echo.Context) error {
		localTime := time.Now()
		utcTime := localTime.UTC()

		return c.String(http.StatusOK, "OK Connected! Local Time: "+localTime.Format(time.RFC3339)+"; UTC Time: "+utcTime.Format(time.RFC3339))
	})

	log.Println("BaseURL: " + a.baseUrl)
}
