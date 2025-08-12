package job

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
)

type FileActivities struct {
	logger zerolog.Logger
	fs     afero.Fs
}

func (a *FileActivities) ListByPattern(ctx context.Context, pattern []string) ([]string, error) {
	var files []string
	if err := afero.Walk(a.fs, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		for _, p := range pattern {
			re, err := regexp.Compile(p)
			if err != nil {
				return err
			}
			if re.MatchString(info.Name()) {
				files = append(files, path)
				break // No need to check other patterns if one matches
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	slices.Sort(files)
	return files, nil
}

func (a *FileActivities) Delete(ctx context.Context, path string) error {
	// if err := a.fs.Remove(path); err != nil {
	// 	a.logger.Warn().Ctx(ctx).Err(err).Str("path", path).Msg("failed to delete file")
	// 	return err
	// }

	a.logger.Info().Ctx(ctx).Str("path", path).Msg("file deleted successfully")

	return nil
}

func NewFileActivities(fs afero.Fs) *FileActivities {
	return &FileActivities{
		logger: log.With().Str("logger", "fileActivity").Logger(),
		fs:     fs,
	}
}

func FileGarbageCollectionWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    15 * time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})

	var fileActivities *FileActivities

	var garbageFiles []string
	if err := workflow.ExecuteActivity(ctx, fileActivities.ListByPattern, viper.GetStringSlice(config.KeyFileGarbageCollectionPattern)).Get(ctx, &garbageFiles); err != nil {
		return fmt.Errorf("failed to get garbage files: %s", err)
	}

	for _, file := range garbageFiles {
		if err := workflow.ExecuteActivity(ctx, fileActivities.Delete, file).Get(ctx, nil); err != nil {
			return fmt.Errorf("failed to delete file: %s", err)
		}
	}

	return nil
}

func RegisterFileGarbageCollectionWorkflow(lc fx.Lifecycle, c client.Client, w worker.Worker, fs afero.Fs) error {
	w.RegisterActivity(&FileActivities{
		logger: log.With().Str("logger", "fileActivities").Logger(),
		fs:     fs,
	})
	w.RegisterWorkflow(FileGarbageCollectionWorkflow)

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	var s client.ScheduleHandle
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			scheduleHandle, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
				ID: hostname,
				Spec: client.ScheduleSpec{
					Intervals: []client.ScheduleIntervalSpec{
						{
							Every: 5 * time.Second,
						},
					},
				},
				Action: &client.ScheduleWorkflowAction{
					ID:        uuid.New().String(),
					Workflow:  FileGarbageCollectionWorkflow,
					TaskQueue: viper.GetString(config.KeyTemporalTaskQueue),
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create schedule: %w", err)
			}
			s = scheduleHandle
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.Delete(ctx)
		},
	})

	return nil
}
