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
	KeyHTTPIdleTimeout     = "http.idle_timeout"
	KeyHTTPShutdownTimeout = "http.shutdown_timeout"

	KeyFileRoot                     = "file.root"
	KeyFileGarbageCollectionPattern = "file.garbage_collection_pattern"
	KeyFileDatabase                 = "file.database"
	KeyFileWebRoot                  = "file.web_root"
	KeyFileWebUploadPath            = "file.web_upload_path"

	KeyTemporalAddress   = "temporal.address"
	KeyTemporalNamespace = "temporal.namespace"
	KeyTemporalTaskQueue = "temporal.task_queue"
)
