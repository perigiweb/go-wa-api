package service

import (
	"log"
	"time"

	"go.mau.fi/whatsmeow/store/sqlstore"

	"github.com/golang-jwt/jwt/v5"
	"github.com/perigiweb/go-wa-api/internal"
	"github.com/perigiweb/go-wa-api/internal/store"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

type Service struct {
	Repo        *store.Repo
	waDataStore *sqlstore.Container
}

func NewService(repo *store.Repo, waDataStore *sqlstore.Container) *Service {
	return &Service{
		Repo:        repo,
		waDataStore: waDataStore,
	}
}

func (s *Service) StartUp() {
	devices, err := s.Repo.GetConnectedDevices()
	if err == nil {
		for _, device := range devices {
			s.WhatsAppReconnect(&device)
		}
	}
}

func (s *Service) SignIn(
	usrEmail string,
	usrPassword string,
	userAgent string,
	ipAddress string,
) (
	status bool,
	message string,
	accessToken string,
	refreshToken string,
) {
	var (
		err         error
		user        entity.User
		userSession entity.UserSession
	)

	user, err = s.Repo.GetUserByEmail(usrEmail)
	if err != nil {
		return false, "Email not registered.", "", ""
	}

	passwordMatch := internal.PasswordVerify(usrPassword, user.Password)
	if !passwordMatch {
		return false, "Email and Password doesnot match.", "", ""
	}

	userSession, err = s.Repo.InsertNewUserSession(user.Id, userAgent, ipAddress)
	if err != nil {
		return false, err.Error(), "", ""
	}

	accessToken, refreshToken, err = internal.CreateJWTToken(user, userSession)

	if err != nil {
		return false, err.Error(), "", ""
	}

	return true, "", accessToken, refreshToken
}

func (s *Service) SignInWithRefreshToken(authRefreshToken *jwt.Token) (status bool, message string, accessToken string, refreshToken string) {
	var (
		err            error
		userSession    entity.UserSession
		newUserSession entity.UserSession
		user           entity.User
	)

	claims := authRefreshToken.Claims.(*internal.RefreshTokenJWTClaims)

	userSession, err = s.Repo.GetUserSessionById(claims.Session)
	if err != nil {
		log.Printf("DEBUG: %s", err.Error())
		return false, "Can't find session with refresh token", "", ""
	}

	exp := time.Unix(userSession.ExpiredAt, 0)
	now := time.Now()
	if now.After(exp) {
		return false, "RefreshToken expired.", "", ""
	}

	user, err = s.Repo.GetUserById(userSession.UserId)
	if err != nil {
		log.Printf("DEBUG: %s", err.Error())
		return false, "Can't find user in session", "", ""
	}

	_ = s.Repo.DeleteUserSessionById(userSession.Id)

	newUserSession, err = s.Repo.InsertNewUserSession(user.Id, userSession.UserAgent, userSession.IpAddress)
	if err != nil {
		log.Printf("DEBUG: %s", err.Error())
		return false, "Can't create new session", "", ""
	}

	accessToken, refreshToken, err = internal.CreateJWTToken(user, newUserSession)

	if err != nil {
		log.Printf("DEBUG: %s", err.Error())
		return false, "Can't create JWT Token", "", ""
	}

	return true, "", accessToken, refreshToken
}

func (s *Service) SignOut(sessionId string) error {
	err := s.Repo.DeleteUserSessionById(sessionId)

	return err
}
