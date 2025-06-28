package job

import (
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/wei840222/simple-file-server/config"
	"github.com/wei840222/simple-file-server/server/handler"
)

type ExpireUploadJob struct {
	logger zerolog.Logger
	db     *gorm.DB
	fs     afero.Fs
}

func (j *ExpireUploadJob) Run() {
	var uploads []handler.Upload
	if err := j.db.Where("expired_at < ?", time.Now()).Find(&uploads).Error; err != nil {
		panic(err)
	}

	for _, upload := range uploads {
		path := upload.ID + upload.FileExtension
		j.logger.Info().Str("path", path).Msg("deleting expired upload")
		if err := j.db.Delete(&upload).Error; err != nil {
			j.logger.Error().Err(err).Str("id", upload.ID).Msg("failed to delete expired upload")
			continue
		}

		if err := j.fs.Remove(path); err != nil {
			j.logger.Warn().Err(err).Str("path", path).Msg("failed to remove expired file")
		}
	}
}

func RegisterExpireUploadJob(c *cron.Cron) error {
	logger := log.With().Str("logger", "gorm").Logger()

	db, err := gorm.Open(sqlite.Open(viper.GetString(config.KeyFileDatabase)), &gorm.Config{
		Logger: gormlogger.New(
			&logger,
			gormlogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormlogger.Info,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      false,
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

	j := ExpireUploadJob{
		logger: log.With().Str("logger", "expireUploadJob").Logger(),
		db:     db,
		fs:     afero.NewBasePathFs(afero.NewOsFs(), viper.GetString(config.KeyFileRoot)),
	}

	if _, err := c.AddJob("@every 5s", &j); err != nil {
		return err
	}

	return nil
}
