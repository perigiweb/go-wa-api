package internal

import (
	"strings"

	"github.com/labstack/echo/v4"
)

func HttpRealIP() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ipAddress := c.Request().RemoteAddr
			if XForwardedFor := c.Request().Header.Get("X-Forwarded-For"); XForwardedFor != "" {
				dataIndex := strings.Index(XForwardedFor, ", ")
				if dataIndex == -1 {
					dataIndex = len(XForwardedFor)
				}

				ipAddress = XForwardedFor[:dataIndex]
			} else if XRealIP := c.Request().Header.Get("X-Real-IP"); XRealIP != "" {
				ipAddress = XRealIP
			}

			i := strings.Index(ipAddress, ":")
			if i == -1 {
				i = len(ipAddress)
			}

			realIP := ipAddress[:i]
			c.Request().RemoteAddr = realIP

			return next(c)
		}
	}
}
