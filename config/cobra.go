package config

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	FlagReplacer = strings.NewReplacer(".", "-", "_", "-")

	AllKeys = []string{
		KeyLogLevel,
		KeyLogFormat,
		KeyLogColor,

		KeyO11yHost,
		KeyO11yPort,

		KeyGinMode,

		KeyHTTPPort,
		KeyHTTPHost,
		KeyHTTPEnableCORS,
		KeyHTTPEnableAuth,
		KeyHTTPReadOnlyTokens,
		KeyHTTPReadWriteTokens,
		KeyHTTPMaxUploadSize,
		KeyHTTPReadTimeout,
		KeyHTTPWriteTimeout,
		KeyHTTPIdleTimeout,
		KeyHTTPShutdownTimeout,

		KeyFileRoot,
		KeyFileGarbageCollectionPattern,
		KeyFileWebRoot,
		KeyFileWebUploadPath,

		KeyTemporalAddress,
		KeyTemporalNamespace,
		KeyTemporalTaskQueue,
	}
)

func InitCobraPFlag(cmd *cobra.Command) {
	for _, key := range AllKeys {
		viper.BindPFlag(key, cmd.Flags().Lookup(FlagReplacer.Replace(key)))
	}
}
