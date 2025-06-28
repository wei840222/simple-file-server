package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"

	"github.com/wei840222/simple-file-server/config"
	"github.com/wei840222/simple-file-server/server"
	"github.com/wei840222/simple-file-server/server/middleware"
)

type FileHandler struct {
	logger zerolog.Logger
	fs     afero.Fs
}

func (h *FileHandler) ServeContent(c *gin.Context) {
	path := strings.TrimPrefix(c.Param("path"), "/")
	h.logger.Debug().Str("path", path).Msg("checking if file exists")

	if path == "" {
		c.Error(server.ErrFileNotFound)
		c.AbortWithStatusJSON(http.StatusNotFound, server.ErrorRes{
			Error: server.ErrFileNotFound.Error(),
		})
		return
	}

	f, err := h.fs.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Error(server.ErrFileNotFound)
			c.AbortWithStatusJSON(http.StatusNotFound, server.ErrorRes{
				Error: server.ErrFileNotFound.Error(),
			})
			return
		}
		panic(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}

	if fi.IsDir() {
		h.logger.Debug().Str("path", path).Msg("path is a directory")
		c.Error(server.ErrFileNotFound)
		c.AbortWithStatusJSON(http.StatusNotFound, server.ErrorRes{
			Error: server.ErrFileNotFound.Error(),
		})
		return
	}

	name := fi.Name()
	modtime := fi.ModTime()
	http.ServeContent(c.Writer, c.Request, name, modtime, f)
}

func (h *FileHandler) UploadContent(c *gin.Context) {
	path := strings.TrimPrefix(c.Param("path"), "/")
	if path == "" {
		c.Error(server.ErrFilePathInvalid)
		c.AbortWithStatusJSON(http.StatusBadRequest, server.ErrorRes{
			Error: server.ErrFilePathInvalid.Error(),
		})
		return
	}

	// Check if the file already exists.
	exists, err := afero.Exists(h.fs, path)
	if err != nil {
		panic(err)
	}

	// If the request method is PUT, we allow overwriting the file.
	allowOverwrite := c.Request.Method == http.MethodPut
	if exists && !allowOverwrite {
		c.Error(server.ErrFileAlreadyExists)
		c.AbortWithStatusJSON(http.StatusConflict, server.ErrorRes{
			Error: server.ErrFileAlreadyExists.Error(),
		})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, server.ErrorRes{
			Error: err.Error(),
		})
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, server.ErrorRes{
			Error: err.Error(),
		})
		return
	}
	defer f.Close()

	src := http.MaxBytesReader(c.Writer, f, viper.GetInt64(config.KeyHTTPMaxUploadSize))
	defer src.Close()

	// Ensure the directories exist.
	dirsPath := filepath.Dir(path)
	if err := h.fs.MkdirAll(dirsPath, 0755); err != nil {
		panic(err)
	}

	dstFile, err := h.fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer dstFile.Close()

	// Copy the content from the source to the destination file.
	written, err := io.Copy(dstFile, src)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.Error(server.ErrFileSizeLimitExceeded)
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, server.ErrorRes{
				Error: server.ErrFileSizeLimitExceeded.Error(),
			})
			return
		}
		panic(err)
	}
	h.logger.Debug().Str("path", path).Int64("bytes", written).Msg("uploaded file")

	if !exists {
		c.JSON(http.StatusCreated, gin.H{
			"message": "file created successfully",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file overwritten successfully",
	})
}

func RegisterFileHandler(e *gin.Engine) {
	h := FileHandler{
		logger: log.With().Str("logger", "fileHandler").Logger(),
		fs:     afero.NewBasePathFs(afero.NewOsFs(), viper.GetString(config.KeyFileRoot)),
	}

	files := e.Group("/files")
	{
		files.HEAD("/*path", middleware.NewTokenAuth(viper.GetStringSlice(config.KeyHTTPReadOnlyTokens)), h.ServeContent)
		files.GET("/*path", middleware.NewTokenAuth(viper.GetStringSlice(config.KeyHTTPReadOnlyTokens)), h.ServeContent)
		files.POST("/*path", middleware.NewTokenAuth(viper.GetStringSlice(config.KeyHTTPReadWriteTokens)), h.UploadContent)
		files.PUT("/*path", middleware.NewTokenAuth(viper.GetStringSlice(config.KeyHTTPReadWriteTokens)), h.UploadContent)
	}
}
