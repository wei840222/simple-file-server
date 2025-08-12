package job

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
)

type temporalLogger struct {
	log zerolog.Logger
}

func (l *temporalLogger) withKeyvals(event *zerolog.Event, keyvals ...any) *zerolog.Event {
	if len(keyvals)%2 != 0 {
		event.Any("keyvals", keyvals).Msg("Called with keyvals, but with wrong number of arguments")
	}
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			event.Any("key", keyvals[i]).Msg("Called with keyvals, but with wrong key type")
			continue
		}
		if err, ok := keyvals[i+1].(error); ok && key == "Error" {
			event = event.Err(err)
		} else {
			event = event.Any(strings.ToLower(key[:1])+key[1:], keyvals[i+1])
		}
	}
	return event
}

func (l *temporalLogger) Debug(msg string, keyvals ...any) {
	l.withKeyvals(l.log.Debug(), keyvals...).Msg(msg)
}

func (l *temporalLogger) Info(msg string, keyvals ...any) {
	l.withKeyvals(l.log.Info(), keyvals...).Msg(msg)
}

func (l *temporalLogger) Warn(msg string, keyvals ...any) {
	l.withKeyvals(l.log.Warn(), keyvals...).Msg(msg)
}

func (l *temporalLogger) Error(msg string, keyvals ...any) {
	l.withKeyvals(l.log.Error(), keyvals...).Msg(msg)
}

func NewTemporalClient(lc fx.Lifecycle) (client.Client, error) {
	log.With().Str("logger", "gorm").Logger()
	c, err := client.Dial(client.Options{
		HostPort:  viper.GetString(config.KeyTemporalAddress),
		Namespace: viper.GetString(config.KeyTemporalNamespace),
		Logger:    &temporalLogger{log.With().Str("logger", "temporalClient").Logger()},
	})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			c.Close()
			return nil
		},
	})

	return c, nil
}

func NewTemporalWorker(lc fx.Lifecycle, c client.Client) (worker.Worker, error) {
	w := worker.New(c, viper.GetString(config.KeyTemporalTaskQueue), worker.Options{})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go w.Run(nil)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			w.Stop()
			return nil
		},
	})

	return w, nil
}
