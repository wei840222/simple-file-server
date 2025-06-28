package handler

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/wei840222/simple-file-server/config"
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
	db *gorm.DB
	fs afero.Fs
}

func (h *UploadHandler) UploadContent(c *gin.Context) {
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

	upload := Upload{
		ID:            id,
		FileExtension: fileExtension,
		CreatedAt:     time.Now(),
		ExpiredAt:     time.Now().Add(7 * 24 * time.Hour),
	}

	if err := h.db.WithContext(c).Create(&upload).Error; err != nil {
		panic(err)
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

	path := id + fileExtension
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
	log.Debug().Str("path", path).Int64("bytes", written).Msg("uploaded file")

	c.JSON(http.StatusCreated, gin.H{
		"message": "file created successfully",
		"path":    path,
	})
}

func RegisterUploadHandler(e *gin.Engine, _ metric.MeterProvider) error {
	db, err := gorm.Open(sqlite.Open(viper.GetString(config.KeyFileDatabase)), &gorm.Config{
		Logger: logger.New(
			&log.Logger,
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Info,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return err
	}
	if err := db.Use(tracing.NewPlugin()); err != nil {
		return err
	}
	if err := db.AutoMigrate(&Upload{}); err != nil {
		return err
	}

	h := &UploadHandler{
		db: db,
		fs: afero.NewBasePathFs(afero.NewOsFs(), viper.GetString(config.KeyFileRoot)),
	}

	e.POST("/upload", middleware.NewTokenAuth(viper.GetStringSlice(config.KeyHTTPReadWriteTokens)), h.UploadContent)

	return nil
}
