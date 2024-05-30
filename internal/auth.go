package internal

import (
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
	"golang.org/x/crypto/bcrypt"
)

type AuthJWTClaims struct {
	UserId  int    `json:"userId"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Session string `json:"session"`
	jwt.RegisteredClaims
}

type RefreshTokenJWTClaims struct {
	Session string `json:"session"`
	jwt.RegisteredClaims
}

func PasswordHash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func PasswordVerify(password string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func TokenPayload(c echo.Context) *AuthJWTClaims {
	user := c.Get("authToken").(*jwt.Token)
	claims := user.Claims.(*AuthJWTClaims)

	return claims
}

func CreateJWTToken(user entity.User, userSession entity.UserSession) (accessToken string, refreshToken string, err error) {
	authClaims := AuthJWTClaims{
		UserId:  user.Id,
		Name:    user.Name,
		Email:   user.Email,
		Session: userSession.Id,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(4 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "PerigiWAWeb",
		},
	}

	refreshTokenClaims := RefreshTokenJWTClaims{
		Session: userSession.Id,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 30 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "PerigiWAWeb",
		},
	}

	authJWTSecret, _ := GetEnvString("JWT_SECRET")
	authJWTRTSecret, _ := GetEnvString("JWT_RT_SECRET")

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, authClaims)
	accessToken, err = jwtToken.SignedString([]byte(authJWTSecret))

	if err != nil {
		log.Printf("CreateJWTToken Error: %s", err.Error())
		return "", "", err
	}

	jwtRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshToken, err = jwtRefreshToken.SignedString([]byte(authJWTRTSecret))

	if err != nil {
		log.Printf("CreateJWTRefreshToken Error: %s", err.Error())
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
