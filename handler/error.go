package handler

import "errors"

var (
	ErrFileNotFound          = errors.New("file not found")
	ErrFilePathInvalid      = errors.New("file path is invalid")
	ErrFileAlreadyExists     = errors.New("file already exists")
	ErrFileSizeLimitExceeded = errors.New("file size limit exceeded")
)

type ErrorRes struct {
	Error string `json:"error"`
}
