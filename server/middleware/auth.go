package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wei840222/simple-file-server/server"
)

func NewTokenAuth(allowedTokens []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(allowedTokens) == 0 {
			// If no tokens are configured, skip authentication.
			c.Next()
			return
		}

		// Extract the token from the Authorization header.
		// The header should be in the format "Bearer <token>"
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			c.Error(server.ErrAuthTokenRequired)
			c.AbortWithStatusJSON(http.StatusUnauthorized, server.ErrorRes{
				Error: server.ErrAuthTokenRequired.Error(),
			})
			return
		}

		// Check if the token is in the list of allowed tokens.
		for _, allowedToken := range allowedTokens {
			if token == allowedToken {
				c.Next()
				return
			}
		}

		// If the token is not in the list, return an error.
		c.Error(server.ErrAuthTokenInvalid)
		c.AbortWithStatusJSON(http.StatusForbidden, server.ErrorRes{
			Error: server.ErrAuthTokenInvalid.Error(),
		})
	}
}
