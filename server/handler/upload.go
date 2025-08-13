package handler

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/metric"
	"go.temporal.io/sdk/client"

	"github.com/wei840222/simple-file-server/config"
	"github.com/wei840222/simple-file-server/job"
	"github.com/wei840222/simple-file-server/server"
	"github.com/wei840222/simple-file-server/server/middleware"
)

func generateRandomID(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}

type Upload struct {
	ID            string    `gorm:"column:id; type:VARCHAR(8); primaryKey"`
	FileExtension string    `gorm:"column:file_extension; type:VARCHAR(8); not null"`
	CreatedAt     time.Time `gorm:"column:created_at; not null"`
	ExpiredAt     time.Time `gorm:"column:expired_at; not null"`
}

type UploadHandler struct {
	logger         zerolog.Logger
	fs             afero.Fs
	temporalClient client.Client
}

func (h *UploadHandler) UploadContent(c *gin.Context) {
	// Extract the expiration time from the query parameters, defaulting to 168 hours (7 days).
	expire, err := time.ParseDuration(c.DefaultQuery("expire", "168h"))
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, server.ErrorRes{
			Error: err.Error(),
		})
		return
	}

	// Validate the expiration time.
	if expire < 1*time.Minute || expire > 30*24*time.Hour {
		c.Error(server.ErrInvalidExpireTime)
		c.AbortWithStatusJSON(http.StatusBadRequest, server.ErrorRes{
			Error: server.ErrInvalidExpireTime.Error(),
		})
		return
	}

	// Generate a random ID for the upload.
	id, err := generateRandomID(8)
	if err != nil {
		panic(err)
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, server.ErrorRes{
			Error: err.Error(),
		})
		return
	}

	fileExtension := filepath.Ext(fh.Filename)
	path := id + fileExtension

	// Check if the error indicates that the file does not exist
	if _, err := h.fs.Stat(path); err == nil {
		panic(fmt.Errorf("file '%s' already exists", path))
	} else if !errors.Is(err, os.ErrNotExist) {
		// Handle other potential errors (e.g., permissions issues)
		panic(fmt.Errorf("error checking file '%s': %w", path, err))
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

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue:  viper.GetString(config.KeyTemporalTaskQueue),
		StartDelay: expire + 5*time.Minute,
	}
	if _, err := h.temporalClient.ExecuteWorkflow(c, workflowOptions, job.FileExpireWorkflow, path); err != nil {
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

	if pathOverwrite := c.GetHeader("X-Path-Overwrite"); pathOverwrite != "" {
		path = server.JoinURL(pathOverwrite, path)
	} else {
		path = server.JoinURL(viper.GetString(config.KeyFileWebUploadPath), path)
	}

	h.logger.Debug().Ctx(c).Str("path", path).Int64("bytes", written).Msg("uploaded file")

	c.JSON(http.StatusCreated, gin.H{
		"message": "file created successfully",
		"path":    path,
	})
}

func RegisterUploadHandler(e *gin.Engine, _ metric.MeterProvider, fs afero.Fs, c client.Client) error {
	h := UploadHandler{
		logger:         log.With().Str("logger", "uploadHandler").Logger(),
		fs:             fs,
		temporalClient: c,
	}

	e.POST("/upload", middleware.NewTokenAuth(viper.GetStringSlice(config.KeyHTTPReadWriteTokens)), h.UploadContent)

	return nil
}
