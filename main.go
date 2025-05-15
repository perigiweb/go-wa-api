package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	//	"strings"
	"log"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTrans "github.com/go-playground/validator/v10/translations/en"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "modernc.org/sqlite"

	"github.com/perigiweb/go-wa-api/internal"
	"github.com/perigiweb/go-wa-api/internal/action"
	"github.com/perigiweb/go-wa-api/internal/service"
	"github.com/perigiweb/go-wa-api/internal/store"
)

type Server struct {
	Address string
	Port    string
}

type EchoValidator struct {
	Validator  *validator.Validate
	Translator ut.Translator
}

var (
	baseUrl   string
	container *sqlstore.Container

	//killchannel = make(map[int](chan bool))
)

func init() {
	var err error
	baseUrl, err = internal.GetEnvString("BASE_URL")
	if err != nil {
		baseUrl = ""
	}
}

func (ev *EchoValidator) Validate(i interface{}) error {
	err := ev.Validator.Struct(i)
	if err != nil {
		errs := err.(validator.ValidationErrors)

		type m struct {
			Message validator.ValidationErrorsTranslations `json:"message"`
		}
		x := &m{
			Message: errs.Translate(ev.Translator),
		}

		return echo.NewHTTPError(http.StatusUnprocessableEntity, x)
	}

	return nil
}

func main() {
	var err error

	c, _ := gocron.NewScheduler()
	defer func() { _ = c.Shutdown() }()

	e := echo.New()

	e.Use(middleware.Recover())

	// Router CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.PATCH, echo.DELETE},
	}))

	// Router Security
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		ContentTypeNosniff: "nosniff",
		XSSProtection:      "1; mode=block",
		XFrameOptions:      "SAMEORIGIN",
	}))

	e.Use(internal.HttpRealIP())

	enLocale := en.New()
	uniTrans := ut.New(enLocale, enLocale)
	translator, _ := uniTrans.GetTranslator("en")
	v := validator.New(validator.WithRequiredStructEnabled())
	enTrans.RegisterDefaultTranslations(v, translator)
	e.Validator = &EchoValidator{
		Validator:  v,
		Translator: translator,
	}

	dbConn := internal.DBConnect()
	defer dbConn.Close()

	waDbLog := waLog.Stdout("Database", "DEBUG", true)
	container = sqlstore.NewWithDB(dbConn, "pgx", waDbLog)
	err = container.Upgrade()
	if err != nil {
		panic(err)
	}

	r := store.NewRepo(dbConn)
	err = r.Migrate()
	if err != nil {
		panic(err)
	}

	s := service.NewService(r, container)
	a := action.NewAction(baseUrl, s)

	a.Routes(e)

	s.StartUp()
	s.CronJobs(c)

	// Get Server Configuration
	var serverConfig Server

	serverConfig.Address, err = internal.GetEnvString("SERVER_ADDRESS")
	if err != nil {
		serverConfig.Address = "127.0.0.1"
	}

	serverConfig.Port, err = internal.GetEnvString("SERVER_PORT")
	if err != nil {
		serverConfig.Port = "3033"
	}

	// Start Server
	go func() {
		err := e.Start(serverConfig.Address + ":" + serverConfig.Port)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err.Error())
		}
	}()

	// Watch for Shutdown Signal
	sigShutdown := make(chan os.Signal, 1)
	signal.Notify(sigShutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sigShutdown

	// Wait 5 Seconds Before Graceful Shutdown
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	// Try To Shutdown Server
	err = e.Shutdown(ctxShutdown)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Try To Shutdown Cron
	//c.Stop()
}
