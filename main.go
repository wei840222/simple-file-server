package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ipfans/fxlogger"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
	"github.com/wei840222/simple-file-server/job"
	"github.com/wei840222/simple-file-server/server"
	"github.com/wei840222/simple-file-server/server/handler"
)

var rootCmd = &cobra.Command{
	Use:   config.AppName,
	Short: "Simple HTTP server to save files.",
	Long:  "Simple HTTP server to save files. With auth support.",
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		logger := log.With().Str("logger", "cobra").Logger()

		if err := config.InitViper(); err != nil {
			return err
		}

		config.InitCobraPFlag(cmd)
		config.InitZerolog()

		if viper.GetBool(config.KeyHTTPEnableAuth) && len(viper.GetStringSlice(config.KeyHTTPReadOnlyTokens)) == 0 && len(viper.GetStringSlice(config.KeyHTTPReadWriteTokens)) == 0 {
			logger.Info().Msg("authentication is enabled but no tokens provided. generating random tokens")
			readOnlyToken, err := generateToken()
			if err != nil {
				return err
			}
			readWriteToken, err := generateToken()
			if err != nil {
				return err
			}
			viper.Set(config.KeyHTTPReadOnlyTokens, readOnlyToken)
			viper.Set(config.KeyHTTPReadWriteTokens, readWriteToken)
			logger.Info().Msgf("generated read only token: %s", readOnlyToken)
			logger.Info().Msgf("generated read write token: %s", readWriteToken)
		}

		logger.Info().Any("config", viper.AllSettings()).Msg("config loaded")

		return nil
	},
	Run: func(*cobra.Command, []string) {
		app := fx.New(
			fx.Provide(
				server.NewMeterProvider,
				server.NewTracerProvider,
				server.NewGinEngine,
				job.NewCronjob,
			),
			fx.Invoke(
				server.RunO11yHTTPServer,
				handler.RegisterFileHandler,
				handler.RegisterUploadHandler,
				job.RegisterExpireUploadJob,
			),
			fx.WithLogger(fxlogger.WithZerolog(log.With().Str("logger", "fx").Logger())),
			fx.StopTimeout(3*viper.GetDuration(config.KeyHTTPShutdownTimeout)),
		)

		app.Run()
	},
}

func main() {
	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyLogLevel), "debug", "Log level")
	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyLogFormat), "console", "Log format")
	rootCmd.PersistentFlags().Bool(config.FlagReplacer.Replace(config.KeyLogColor), true, "Log color")

	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyO11yHost), "0.0.0.0", "Observability server host")
	rootCmd.PersistentFlags().Int(config.FlagReplacer.Replace(config.KeyO11yPort), 9090, "Observability server port")

	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyGinMode), "debug", "Gin mode")

	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyHTTPHost), "0.0.0.0", "HTTP server host")
	rootCmd.PersistentFlags().Int(config.FlagReplacer.Replace(config.KeyHTTPPort), 8080, "HTTP server port")
	rootCmd.PersistentFlags().Bool(config.FlagReplacer.Replace(config.KeyHTTPEnableCORS), false, "Enable CORS header")
	rootCmd.PersistentFlags().Bool(config.FlagReplacer.Replace(config.KeyHTTPEnableAuth), false, "Enable authentication")
	rootCmd.PersistentFlags().StringSlice(config.FlagReplacer.Replace(config.KeyHTTPReadOnlyTokens), []string{}, "Comma separated list of read only tokens")
	rootCmd.PersistentFlags().StringSlice(config.FlagReplacer.Replace(config.KeyHTTPReadWriteTokens), []string{}, "Comma separated list of read write tokens")
	rootCmd.PersistentFlags().Int64(config.FlagReplacer.Replace(config.KeyHTTPMaxUploadSize), 5242880, "Maximum upload size in bytes")
	rootCmd.PersistentFlags().Duration(config.FlagReplacer.Replace(config.KeyHTTPReadTimeout), 15*time.Second, "Read timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms').")
	rootCmd.PersistentFlags().Duration(config.FlagReplacer.Replace(config.KeyHTTPWriteTimeout), 300*time.Second, "Write timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms').")
	rootCmd.PersistentFlags().Duration(config.FlagReplacer.Replace(config.KeyHTTPIdleTimeout), 60*time.Second, "Idle timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms').")
	rootCmd.PersistentFlags().Duration(config.FlagReplacer.Replace(config.KeyHTTPShutdownTimeout), 15*time.Second, "Graceful shutdown timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms').")

	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyFileRoot), "./data/files", "Path to save uploaded files.")
	rootCmd.PersistentFlags().String(config.FlagReplacer.Replace(config.KeyFileDatabase), "./data/sqlite.db", "Path to the SQLite database file. If the file does not exist, it will be created.")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
