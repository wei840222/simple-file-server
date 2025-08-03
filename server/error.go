package server

import "errors"

var (
	ErrFileNotFound          = errors.New("file not found")
	ErrFilePathInvalid       = errors.New("file path is invalid")
	ErrFileAlreadyExists     = errors.New("file already exists")
	ErrFileSizeLimitExceeded = errors.New("file size limit exceeded")

	ErrAuthTokenRequired = errors.New("authorization token is required")
	ErrAuthTokenInvalid  = errors.New("invalid authorization token")

	ErrInvalidExpireTime = errors.New("invalid expiration time")
)

type ErrorRes struct {
	Error string `json:"error"`
}
