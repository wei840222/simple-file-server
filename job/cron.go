package job

import (
	"context"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

func NewCronjob(lc fx.Lifecycle) *cron.Cron {
	logger := log.With().Str("logger", "cron").Logger()

	c := cron.New(
		cron.WithParser(cron.NewParser(
			cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
		)),
		cron.WithChain(
			cron.Recover(cron.VerbosePrintfLogger(&logger)),
			cron.SkipIfStillRunning(cron.VerbosePrintfLogger(&logger)),
		),
		cron.WithLogger(cron.VerbosePrintfLogger(&logger)),
	)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			c.Start()
			return nil
		},
		OnStop: func(context.Context) error {
			c.Stop()
			return nil
		},
	})

	return c
}
