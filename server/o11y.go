package server

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	_ "github.com/grafana/pyroscope-go/godeltaprof/http/pprof"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
)

func NewTracerProvider(lc fx.Lifecycle) (trace.TracerProvider, error) {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.AppName),
		),
	)
	if err != nil {
		return nil, err
	}

	exp, err := otlptracegrpc.New(context.Background())
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
	ptp := otelpyroscope.NewTracerProvider(tp)
	otel.SetTracerProvider(otelpyroscope.NewTracerProvider(ptp))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	})

	return ptp, nil
}

func NewMeterProvider(lc fx.Lifecycle) (metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	otel.SetMeterProvider(provider)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return provider.Shutdown(ctx)
		},
	})

	return provider, nil
}

func RunO11yHTTPServer(lc fx.Lifecycle) {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", viper.GetString(config.KeyO11yHost), viper.GetInt(config.KeyO11yPort)),
		Handler: mux,
	}

	var isShuttingDown bool
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if !isShuttingDown {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service is shutting down"))
		}
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

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
			isShuttingDown = true
			return srv.Shutdown(ctx)
		},
	})

}
