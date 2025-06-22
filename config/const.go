package config

const (
	AppName  = "simple-file-server"
	FileName = "config"

	KeyLogLevel  = "log.level"
	KeyLogFormat = "log.format"
	KeyLogColor  = "log.color"

	KeyO11yHost = "o11y.host"
	KeyO11yPort = "o11y.port"

	KeyGinMode = "gin.mode"

	KeyHTTPPort            = "http.port"
	KeyHTTPHost            = "http.host"
	KeyHTTPEnableCORS      = "http.enable_cors"
	KeyHTTPEnableAuth      = "http.enable_auth"
	KeyHTTPReadOnlyTokens  = "http.read_only_tokens"
	KeyHTTPReadWriteTokens = "http.read_write_tokens"
	KeyHTTPMaxUploadSize   = "http.max_upload_size"
	KeyHTTPReadTimeout     = "http.read_timeout"
	KeyHTTPWriteTimeout    = "http.write_timeout"
	KeyHTTPShutdownTimeout = "http.shutdown_timeout"

	KeyFileRoot           = "file.root"
	KeyFileNamingStrategy = "file.naming_strategy"
)

var AllKeys = []string{
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
	KeyHTTPShutdownTimeout,

	KeyFileRoot,
	KeyFileNamingStrategy,
}
