package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ipfans/fxlogger"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
	simpleuploadserver "github.com/wei840222/simple-file-server/pkg"
)

var (
	flagReplacer = strings.NewReplacer(".", "-")
)

var rootCmd = &cobra.Command{
	Use:   config.AppName,
	Short: "Simple HTTP server to save files.",
	Long:  "Simple HTTP server to save files. With auth support.",
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if err := config.InitViper(); err != nil {
			return err
		}

		for _, key := range config.AllKeys {
			viper.BindPFlag(key, cmd.Flags().Lookup(flagReplacer.Replace(key)))
		}

		config.InitZerolog()

		if viper.GetBool(config.KeyHTTPEnableAuth) && len(viper.GetStringSlice(config.KeyHTTPReadOnlyTokens)) == 0 && len(viper.GetStringSlice(config.KeyHTTPReadWriteTokens)) == 0 {
			log.Info().Msg("authentication is enabled but no tokens provided. generating random tokens")
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
			log.Info().Msgf("generated read only token: %s", readOnlyToken)
			log.Info().Msgf("generated read write token: %s", readWriteToken)
		}

		log.Debug().Any("config", viper.AllSettings()).Msg("config loaded")

		return nil
	},
	Run: func(*cobra.Command, []string) {
		app := fx.New(
			fx.Provide(
				simpleuploadserver.NewServer,
			),
			fx.Invoke(
				simpleuploadserver.Start,
			),
			fx.WithLogger(fxlogger.WithZerolog(log.Logger)),
		)

		app.Run()
	},
}

func main() {
	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyLogLevel), "debug", "Log level")
	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyLogFormat), "console", "Log format")
	rootCmd.PersistentFlags().Bool(flagReplacer.Replace(config.KeyLogColor), true, "Log color")

	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyO11yHost), "0.0.0.0", "Observability server host")
	rootCmd.PersistentFlags().Int(flagReplacer.Replace(config.KeyO11yPort), 9090, "Observability server port")

	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyGinMode), "debug", "Gin mode")

	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyHTTPHost), "0.0.0.0", "HTTP server host")
	rootCmd.PersistentFlags().Int(flagReplacer.Replace(config.KeyHTTPPort), 8080, "HTTP server port")
	rootCmd.PersistentFlags().Bool(flagReplacer.Replace(config.KeyHTTPEnableCORS), true, "Enable CORS header")
	rootCmd.PersistentFlags().Bool(flagReplacer.Replace(config.KeyHTTPEnableAuth), false, "Enable authentication")
	rootCmd.PersistentFlags().StringSlice(flagReplacer.Replace(config.KeyHTTPReadOnlyTokens), []string{}, "Comma separated list of read only tokens")
	rootCmd.PersistentFlags().StringSlice(flagReplacer.Replace(config.KeyHTTPReadWriteTokens), []string{}, "Comma separated list of read write tokens")
	rootCmd.PersistentFlags().Int64(flagReplacer.Replace(config.KeyHTTPMaxUploadSize), 5242880, "Maximum upload size in bytes")
	rootCmd.PersistentFlags().Duration(flagReplacer.Replace(config.KeyHTTPReadTimeout), 15*time.Second, "Read timeout. zero or negative value means no timeout. can be suffixed by the time units 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h' (e.g. '1s', '500ms').")
	rootCmd.PersistentFlags().Duration(flagReplacer.Replace(config.KeyHTTPWriteTimeout), 300*time.Second, "Write timeout. zero or negative value means no timeout. can be suffixed by the time units 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h' (e.g. '1s', '500ms').")
	rootCmd.PersistentFlags().Duration(flagReplacer.Replace(config.KeyHTTPShutdownTimeout), 15*time.Second, "Graceful shutdown timeout. zero or negative value means no timeout. can be suffixed by the time units 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h' (e.g. '1s', '500ms').")

	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyFileRoot), ".", "File path to document root directory")
	rootCmd.PersistentFlags().String(flagReplacer.Replace(config.KeyFileNamingStrategy), "uuid", "File naming strategy")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
