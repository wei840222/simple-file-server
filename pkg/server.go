package simpleuploadserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/wei840222/simple-file-server/config"
)

type Server struct {
	fs afero.Fs
}

var (
	FormFileKey       = "file"
	OverwriteQueryKey = "overwrite"
)

var (
	ErrFileSizeLimitExceeded = fmt.Errorf("file size limit exceeded")
)

// NewServer creates a new Server.
func NewServer() *Server {
	return &Server{
		afero.NewBasePathFs(afero.NewOsFs(), viper.GetString(config.KeyFileRoot)),
	}
}

// Start starts listening on `addr`. This function blocks until the server is stopped.
// Optionally you can pass a channel to `ready` to be notified when the server is ready to accept connections. You can pass nil if you don't need it.
func Start(lc fx.Lifecycle, s *Server) {
	r := mux.NewRouter()
	r.HandleFunc("/upload", s.handle(s.handlePost)).Methods(http.MethodPost)
	r.HandleFunc("/upload", s.handle(s.handleOptions)).Methods(http.MethodOptions)
	// GET handler can handle HEAD request. The difference is that the response body should be empty on HEAD request.
	r.PathPrefix("/files").Methods(http.MethodGet, http.MethodHead).HandlerFunc(s.handle(s.handleGet))
	r.PathPrefix("/files").Methods(http.MethodPut).HandlerFunc(s.handle(s.handlePut))
	r.PathPrefix("/files").Methods(http.MethodOptions).HandlerFunc(s.handle(s.handleOptions))
	r.NotFoundHandler = http.HandlerFunc(handleNotFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(handleMethodNotAllowed)
	if viper.GetBool(config.KeyHTTPEnableAuth) {
		r.Use(s.authenticationMiddleware)
	}
	r.Use(logAccess)

	addr := fmt.Sprintf("%s:%d", viper.GetString(config.KeyHTTPHost), viper.GetInt(config.KeyHTTPPort))
	srv := &http.Server{
		Addr:         addr,
		WriteTimeout: viper.GetDuration(config.KeyHTTPWriteTimeout),
		ReadTimeout:  viper.GetDuration(config.KeyHTTPReadTimeout),
		IdleTimeout:  viper.GetDuration(config.KeyHTTPIdleTimeout),
		Handler:      r,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info().Msgf("Start serving on %s", addr)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info().Msgf("Shutting down... wait up to %v", viper.GetDuration(config.KeyHTTPShutdownTimeout))
			sctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration(config.KeyHTTPShutdownTimeout))
			defer cancel()
			return srv.Shutdown(sctx)
		},
	})
}

func logAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vs := []string{
			r.RemoteAddr,
			"-",
			"-",
			time.Now().Format("[02/Jan/2006:15:04:05 -0700]"),
			fmt.Sprintf("\"%s %s %s\"", r.Method, r.URL.Path, r.Proto),
			fmt.Sprintf("%d", http.StatusOK), // TODO: actual status
			"0",                              // TODO: actual size
			fmt.Sprintf("\"%s\"", r.Referer()),
			fmt.Sprintf("\"%s\"", r.UserAgent()),
		}
		log.Info().Msg(strings.Join(vs, " "))
		next.ServeHTTP(w, r)
	})
}

var fileRe = regexp.MustCompile(`^/files/(.+)$`)

func getPathFromURL(u *url.URL) string {
	matches := fileRe.FindStringSubmatch(u.Path)
	if matches == nil {
		return ""
	}
	return matches[1]
}

type ErrorResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type SuccessfullyUploadedResult struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

func justOK() (int, any) {
	return 0, nil
}

func (s *Server) handle(f func(w http.ResponseWriter, r *http.Request) (int, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, result := f(w, r)
		var responseBody []byte
		if result != nil {
			switch v := result.(type) {
			case error:
				result = ErrorResult{false, v.Error()}
			}
			respBytes, err := json.Marshal(result)
			if err != nil {
				log.Printf("failed to encode response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			responseBody = respBytes
		}
		if responseBody != nil {
			w.Header().Set("Content-Type", "application/json")
			if status != 0 {
				w.WriteHeader(status)
			}
			if _, err := w.Write(responseBody); err != nil {
				log.Printf("failed to write response: %v", err)
			}
		} else {
			if status != 0 {
				w.WriteHeader(status)
			}
		}
	}
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) (int, any) {
	status, destPath, err := s.processUpload(w, r, "")
	if err != nil {
		return status, err
	}
	if viper.GetBool(config.KeyHTTPEnableCORS) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	return http.StatusCreated, SuccessfullyUploadedResult{true, destPath}
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) (int, any) {
	path := getPathFromURL(r.URL)
	if path == "" {
		log.Printf("URL not matched: (url=%s)", r.URL.String())
		return http.StatusMethodNotAllowed, fmt.Errorf("PUT is accepted on /files/:name")
	}

	status, destPath, err := s.processUpload(w, r, path)
	if err != nil {
		return status, err
	}

	if viper.GetBool(config.KeyHTTPEnableCORS) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	return http.StatusCreated, SuccessfullyUploadedResult{true, destPath}
}

func (s *Server) processUpload(w http.ResponseWriter, r *http.Request, path string) (int, string, error) {
	allowOverwrite := parseBoolishValue(r.URL.Query().Get(OverwriteQueryKey))
	if allowOverwrite {
		log.Printf("allowOverwrite")
	}

	srcFile, info, err := r.FormFile(FormFileKey)
	if err != nil {
		log.Printf("failed to obtain form file: %v", err)
		return http.StatusInternalServerError, "", fmt.Errorf("cannot obtain the uploaded content")
	}
	src := http.MaxBytesReader(w, srcFile, viper.GetInt64(config.KeyHTTPMaxUploadSize))
	// MaxBytesReader closes the underlying io.Reader on its Close() is called
	defer src.Close()

	// on POST method request
	if path == "" {
		filename := info.Filename
		if filename == "" {
			namer := ResolveFileNamingStrategy(viper.GetString(config.KeyFileNamingStrategy))
			s, err := namer(srcFile, info)
			if err != nil {
				log.Printf("cannot generate filename: %v", err)
				return http.StatusInternalServerError, "", fmt.Errorf("cannot generate filename")
			}
			filename = s
		}
		path = "/" + filename
	}

	if exists, err := afero.Exists(s.fs, path); err != nil {
		log.Printf("failed to check the existence of the file (path=%s): %v", path, err)
		return http.StatusInternalServerError, "", fmt.Errorf("cannot check the existence of the file")
	} else if exists && !allowOverwrite {
		return http.StatusConflict, "", fmt.Errorf("the file already exists")
	}

	// ensure the directories exist
	dirsPath := filepath.Dir(path)
	if err := s.fs.MkdirAll(dirsPath, 0755); err != nil {
		log.Printf("failed to create directories (path=%s): %v", dirsPath, err)
		return http.StatusInternalServerError, "", fmt.Errorf("cannot create directories")
	}

	dstFile, err := s.fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("failed to open the destination file (path=%s): %v", path, err)
		return http.StatusInternalServerError, "", fmt.Errorf("cannot open file")
	}
	defer dstFile.Close()
	written, err := io.Copy(dstFile, src)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return http.StatusRequestEntityTooLarge, "", ErrFileSizeLimitExceeded
		}
		log.Printf("failed to write the uploaded content: %v", err)
		return http.StatusInternalServerError, "", fmt.Errorf("failed to write the content")
	}
	log.Printf("uploaded to %s (%d bytes)", path, written)

	destPath := path
	if !strings.HasPrefix(destPath, "/") {
		destPath = "/" + destPath
	}
	destPath = "/files" + destPath

	log.Printf("uploaded by PUT to %s (%d bytes)", path, written)
	if viper.GetBool(config.KeyHTTPEnableCORS) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	return http.StatusCreated, destPath, nil
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) (int, any) {
	requestPath := getPathFromURL(r.URL)
	if requestPath == "" {
		return http.StatusNotFound, fmt.Errorf("file not found")
	}
	log.Printf("GET %s -> %s", r.URL.Path, requestPath)
	f, err := s.fs.Open(requestPath)
	if viper.GetBool(config.KeyHTTPEnableCORS) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	if err != nil {
		// ErrNotExist is a common case so don't log it
		if errors.Is(err, os.ErrNotExist) {
			return http.StatusNotFound, fmt.Errorf("file not found")
		}
		log.Printf("Error: %+v", err)
		return http.StatusInternalServerError, fmt.Errorf("failed to open file")
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		log.Printf("failed to stat: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("stat failed")
	}
	if fi.IsDir() {
		// TODO
		log.Printf("%s is a directory", requestPath)
		return http.StatusNotFound, fmt.Errorf("%s is a directory", requestPath)
	}
	name := fi.Name()
	modtime := fi.ModTime()
	http.ServeContent(w, r, name, modtime, f)
	return justOK()
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) (int, any) {
	var allowedMethods []string
	if r.URL.Path == "/upload" {
		allowedMethods = []string{http.MethodPost}
	} else if strings.HasPrefix(r.URL.Path, "/files") {
		allowedMethods = []string{http.MethodGet, http.MethodPut, http.MethodHead}
	}
	if viper.GetBool(config.KeyHTTPEnableCORS) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
	return http.StatusNoContent, nil
}

func (s *Server) authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OPTIONS request is always allowed without authentication
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		var token string
		if auth := r.Header.Get("Authorization"); auth != "" {
			token = strings.TrimPrefix(auth, "Bearer ")
		} else if t := r.URL.Query().Get("token"); t != "" {
			token = t
		}
		if token == "" {
			log.Printf("no token")
			writeUnauthorized(w, r)
			return
		}
		var allowedTokens []string
		allowedTokens = append(allowedTokens, viper.GetStringSlice(config.KeyHTTPReadWriteTokens)...)
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			allowedTokens = append(allowedTokens, viper.GetStringSlice(config.KeyHTTPReadOnlyTokens)...)
		}
		if !slices.Contains(allowedTokens, token) {
			log.Printf("invalid token")
			writeUnauthorized(w, r)
			return
		}
		log.Print("successfully authenticated")
		r.Header.Del("Authorization")
		u := r.URL
		q := u.Query()
		q.Del("token")
		u.RawQuery = q.Encode()
		r.URL = u
		next.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	if r.Method != http.MethodHead {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(http.StatusUnauthorized)
	if r.Method == http.MethodHead {
		return
	}
	resp := ErrorResult{false, "unauthorized"}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	resp := ErrorResult{false, "not found"}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	var endpoint string
	var allowedMethods []string
	if r.URL.Path == "/upload" {
		endpoint = "/upload"
		allowedMethods = []string{http.MethodPost}
	}
	if strings.HasPrefix(r.URL.Path, "/files") {
		endpoint = "/files"
		allowedMethods = []string{http.MethodGet, http.MethodPut}
	}
	w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
	resp := ErrorResult{false, fmt.Sprintf("%s is not allowed on %s", r.Method, endpoint)}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func getFileSize(r io.Seeker) (int64, error) {
	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if _, err := r.Seek(cur, io.SeekStart); err != nil {
		return 0, err
	}
	return size, nil
}

func parseBoolishValue(s string) bool {
	truthyValues := []string{"yes", "true", "1"}
	return slices.Contains(truthyValues, strings.ToLower(s))
}
