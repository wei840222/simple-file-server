# simple-file-server

Simple HTTP server to save files

- [Features](#features)
- [Usage](#usage)
- [Authentication](#authentication)
- [Timeouts](#timeouts)
- [Observability](#observability)
- [File Storage](#file-storage)
- [API](#api)
  - [`POST /upload`](#post-upload)
  - [`POST /files/:path`](#post-filespath)
  - [`PUT /files/:path`](#put-filespath)
  - [`HEAD /files/:path`](#head-filespath)
  - [`GET /files/:path`](#get-filespath)

## Features

- **Simple file upload and download**: Upload files via POST/PUT and download via GET
- **Random filename generation**: `/upload` endpoint generates unique filenames automatically
- **Authentication support**: Token-based authentication with read-only and read-write permissions
- **CORS support**: Enable cross-origin requests when needed
- **Configurable timeouts**: Fine-tune read, write, idle, and shutdown timeouts
- **Observability**: Built-in metrics and tracing support
- **File size limits**: Configurable maximum upload size
- **Graceful shutdown**: Proper cleanup on termination

## Usage

```
      --file-garbage-collection-pattern strings   Regular expressions to match files for garbage collection. Files matching these patterns will be deleted. (default [^\._.+,^\.DS_Store$])
      --file-root string                          Path to save uploaded files. (default "./data/files")
      --file-web-root string                      Path to the web root directory. This is used to serve the static files for the web interface. (default "./web/dist")
      --file-web-upload-path string               Path of the upload api response. (default "./files")
      --gin-mode string                           Gin mode (default "debug")
  -h, --help                                      help for simple-file-server
      --http-enable-auth                          Enable authentication
      --http-enable-cors                          Enable CORS header
      --http-host string                          HTTP server host (default "0.0.0.0")
      --http-idle-timeout duration                Idle timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms'). (default 1m0s)
      --http-max-upload-size int                  Maximum upload size in bytes (default 5242880)
      --http-port int                             HTTP server port (default 8080)
      --http-read-only-tokens strings             Comma separated list of read only tokens
      --http-read-timeout duration                Read timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms'). (default 15s)
      --http-read-write-tokens strings            Comma separated list of read write tokens
      --http-shutdown-timeout duration            Graceful shutdown timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms'). (default 15s)
      --http-write-timeout duration               Write timeout. zero or negative value means no timeout. can be suffixed by the time units (e.g. '1s', '500ms'). (default 5m0s)
      --log-color                                 Log color (default true)
      --log-format string                         Log format (default "console")
      --log-level string                          Log level (default "debug")
      --o11y-host string                          Observability server host (default "0.0.0.0")
      --o11y-port int                             Observability server port (default 9090)
      --temporal-address string                   Temporal server address. (default "localhost:7233")
      --temporal-namespace string                 Temporal namespace. (default "default")
      --temporal-task-queue string                Temporal task queue. (default "SIMPLE_FILE_SERVER:FILES")
```

The server supports configuration via command line flags, environment variables, and configuration files. Command line flags take precedence over environment variables, which take precedence over configuration files.

## Authentication

This server does not require authentication by default. Anyone who can access the server can get/upload files.

The server implements a simple authentication mechanism using tokens.

1. Configure the server to enable authentication: `--http-enable-auth` flag.
2. Prepare tokens. Any string value is valid as a token.
3. Add them to the configuration using `--http-read-only-tokens` and `--http-read-write-tokens` flags.
   If authentication is enabled but no tokens provided, the server generates a read-only token and a read-write token on startup and displays them in the logs.
4. Request with the token. Add Authorization header with value `Bearer <TOKEN>` or `token=<TOKEN>` to the query parameter. Authorization header takes precedence.

| Token Type | Allowed Operations                         |
| ---------- | ------------------------------------------ |
| read-only  | `GET`, `HEAD`                              |
| read-write | `POST`, `PUT` in addition to read-only ops |

Note that `OPTIONS` is always allowed without authentication.

Authentication fails when:

- A request has no tokens.
- A request has a token but not registered to the server.
- A request has a token but not allowed to the requested operation.

In these cases, the server responds with `401 Unauthorized` with body like: `{"error": "unauthorized"}`.

No one can request write operations if you configure the server with read-only tokens only.
As a result, the server operates in read-only mode.

## Observability

The server includes built-in observability features:

- Metrics endpoint available on port 9090 (configurable with `--o11y-port`)
- OpenTelemetry tracing support
- Structured logging with zerolog

## File Storage

Files are stored in the filesystem at the location specified by `--file-root` flag (default: `./data/files`).
The `/upload` endpoint generates unique 8-character IDs for uploaded files, while `/files/:path` endpoints allow you to specify custom paths.

## Timeouts

There are multiple timeout configurations available:

- **Read timeout** (`--http-read-timeout`): Maximum duration for the server reading the request. Clients should finish sending request headers and the entire content within this timeout. Default: 15 seconds.
- **Write timeout** (`--http-write-timeout`): Maximum duration for the server writing the response. Clients should finish downloading the content within this timeout. Default: 5 minutes.
- **Idle timeout** (`--http-idle-timeout`): Maximum duration for idle connections. Default: 1 minute.
- **Shutdown timeout** (`--http-shutdown-timeout`): Maximum duration for graceful shutdown. Default: 15 seconds.

Please consider changing these timeouts if:

- the server or the clients are in a low-bandwidth network.
- you are working with large files.

Note that longer timeouts will result in more connections being maintained.

## API

### `POST /upload`

Uploads a new file with an automatically generated filename. The server generates a random 8-character ID and uses the original file extension.

#### Request

Content-Type
: `multipart/form-data`

Parameters:

| Name     | Required? | Type         | Description              | Default |
| -------- | :-------: | ------------ | ------------------------ | ------- |
| `file`   |     v     | Form Data    | A content of the file.   |         |
| `expire` |     x     | Query String | Expire time of the file. | 168h    |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

| Name      | Type     | Description                             |
| --------- | -------- | --------------------------------------- |
| `message` | `string` | Success message.                        |
| `path`    | `string` | A path to access this file in this API. |

##### On Failure

| StatusCode                     | When                                   |
| ------------------------------ | -------------------------------------- |
| `400 Bad Request`              | Invalid request or missing file field. |
| `413 Request Entity Too Large` | File size exceeds the upload limit.    |

#### Example

```bash
echo 'Hello, world!' > sample.txt
curl -X POST -F file=@sample.txt http://localhost:8080/upload?expire=1h
```

```
{"message":"file created successfully","path":"abc12345.txt"}
```

### `POST /files/:path`

Uploads a file to a specific path. Creates a new file at the specified path.

#### Parameters

| Name    | Required? | Type      | Description            | Default |
| ------- | :-------: | --------- | ---------------------- | ------- |
| `:path` |     v     | `string`  | Path to the file.      |         |
| `file`  |     v     | Form Data | A content of the file. |         |

#### Response

##### On Successful

Status Code
: `201 Created`

Content-Type
: `application/json`

Body:

| Name      | Type     | Description      |
| --------- | -------- | ---------------- |
| `message` | `string` | Success message. |

##### On Failure

| StatusCode                     | When                                           |
| ------------------------------ | ---------------------------------------------- |
| `400 Bad Request`              | Invalid file path or missing file field.       |
| `409 Conflict`                 | There is already a file at the specified path. |
| `413 Request Entity Too Large` | File size exceeds the upload limit.            |

#### Example

```bash
curl -X POST -F file=@sample.txt "http://localhost:8080/files/test/sample.txt"
```

```
{"message":"file created successfully"}
```

### `PUT /files/:path`

Uploads a file to a specific path. Allows overwriting existing files.

#### Parameters

| Name    | Required? | Type      | Description            | Default |
| ------- | :-------: | --------- | ---------------------- | ------- |
| `:path` |     v     | `string`  | Path to the file.      |         |
| `file`  |     v     | Form Data | A content of the file. |         |

#### Response

##### On Successful

Status Code
: `201 Created` (for new files) or `200 OK` (for overwritten files)

Content-Type
: `application/json`

Body:

| Name      | Type     | Description      |
| --------- | -------- | ---------------- |
| `message` | `string` | Success message. |

##### On Failure

| StatusCode                     | When                                     |
| ------------------------------ | ---------------------------------------- |
| `400 Bad Request`              | Invalid file path or missing file field. |
| `413 Request Entity Too Large` | File size exceeds the upload limit.      |

#### Example

```bash
curl -X PUT -F file=@sample.txt "http://localhost:8080/files/foobar.txt"
```

```
{"message":"file created successfully"}
```

### `HEAD /files/:path`

Check existence of a file.

#### Request

Parameters:

| Name    | Required? | Type     | Description         | Default |
| ------- | :-------: | -------- | ------------------- | ------- |
| `:path` |     v     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `200 OK`

Body
: Not Available

##### On Failure

| StatusCode      | When                                               |
| --------------- | -------------------------------------------------- |
| `404 Not Found` | No such file on the server or path is a directory. |

#### Example

```bash
curl -I http://localhost:8080/files/foobar.txt
```

### `GET /files/:path`

Downloads a file.

#### Request

Parameters:

| Name    | Required? | Type     | Description         | Default |
| ------- | :-------: | -------- | ------------------- | ------- |
| `:path` |     v     | `string` | A path to the file. |         |

#### Response

##### On Successful

Status Code
: `200 OK`

Content-Type
: Depends on the content.

Body
: The content of the requested file.

##### On Failure

Content-Type
: `application/json`

| StatusCode      | When                                          |
| --------------- | --------------------------------------------- |
| `404 Not Found` | There is no such file or path is a directory. |

#### Example

```bash
curl http://localhost:8080/files/sample.txt
```

```
Hello, world!
```
