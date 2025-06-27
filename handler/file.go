package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/wei840222/simple-file-server/config"
)

type FileHandler struct {
	fs afero.Fs
}

func (h *FileHandler) ServeContent(c *gin.Context) {
	path := strings.TrimPrefix(c.Param("path"), "/")
	log.Debug().Str("path", path).Msg("checking if file exists")

	if path == "" {
		c.Error(ErrFileNotFound)
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorRes{
			Error: ErrFileNotFound.Error(),
		})
		return
	}

	f, err := h.fs.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Error(ErrFileNotFound)
			c.AbortWithStatusJSON(http.StatusNotFound, ErrorRes{
				Error: ErrFileNotFound.Error(),
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
		log.Debug().Str("path", path).Msg("path is a directory")
		c.Error(ErrFileNotFound)
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorRes{
			Error: ErrFileNotFound.Error(),
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
		c.Error(ErrFilePathInvalid)
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorRes{
			Error: ErrFilePathInvalid.Error(),
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
		c.Error(ErrFileAlreadyExists)
		c.AbortWithStatusJSON(http.StatusConflict, ErrorRes{
			Error: ErrFileAlreadyExists.Error(),
		})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorRes{
			Error: err.Error(),
		})
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorRes{
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
			c.Error(ErrFileSizeLimitExceeded)
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, ErrorRes{
				Error: ErrFileSizeLimitExceeded.Error(),
			})
			return
		}
		panic(err)
	}
	log.Debug().Str("path", path).Int64("bytes", written).Msg("uploaded file")

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
	h := &FileHandler{
		fs: afero.NewBasePathFs(afero.NewOsFs(), viper.GetString(config.KeyFileRoot)),
	}

	files := e.Group("/files")
	{
		files.HEAD("/*path", h.ServeContent)
		files.GET("/*path", h.ServeContent)
		files.POST("/*path", h.UploadContent)
		files.PUT("/*path", h.UploadContent)
	}
}
