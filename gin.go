package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
)

func NewGinLogger(notLogged ...string) gin.HandlerFunc {
	var skip map[string]struct{}

	if length := len(notLogged); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, p := range notLogged {
			skip[p] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		// other handler can change c.Path so:
		path := c.Request.URL.Path
		now := time.Now()
		c.Next()
		latency := time.Since(now)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		clientUserAgent := c.Request.UserAgent()
		referer := c.Request.Referer()
		dataLength := c.Writer.Size()
		if dataLength < 0 {
			dataLength = 0
		}

		if _, ok := skip[path]; ok {
			return
		}

		entry := map[string]any{
			"host":          c.Request.Host,
			"status":        status,
			"latency":       latency.String(),
			"clientIP":      clientIP,
			"method":        c.Request.Method,
			"path":          path,
			"referer":       referer,
			"responseBytes": dataLength,
			"userAgent":     clientUserAgent,
			"traceID":       trace.SpanContextFromContext(c).TraceID(),
		}

		msg := fmt.Sprintf("[GIN] %v | %d | %13v | %s | %s | %-7s %#v",
			now.Format("2006/01/02 - 15:04:05"),
			status,
			latency,
			c.Request.Host,
			clientIP,
			c.Request.Method,
			path,
		)

		if len(c.Errors) > 0 {
			log.Error().Fields(entry).Msg(strings.TrimSpace(c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}
		if status >= http.StatusInternalServerError {
			log.Error().Fields(entry).Msg(msg)
		} else if status >= http.StatusBadRequest {
			log.Warn().Fields(entry).Msg(msg)
		} else {
			log.Info().Fields(entry).Msg(msg)
		}
	}
}

func NewGinEngine(lc fx.Lifecycle, tp trace.TracerProvider) *gin.Engine {
	gin.SetMode(viper.GetString(config.KeyGinMode))

	e := gin.New()
	e.ContextWithFallback = true

	e.Use(otelgin.Middleware(config.AppName, otelgin.WithTracerProvider(tp)), NewGinLogger(), gin.Recovery())

	m := ginmetrics.GetMonitor()
	m.SetSlowTime(1)
	m.SetDuration([]float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 10})
	m.UseWithoutExposingEndpoint(e)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", viper.GetString(config.KeyHTTPHost), viper.GetInt(config.KeyHTTPHost)),
		Handler: e,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})

	return e
}
