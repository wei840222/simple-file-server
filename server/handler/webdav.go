package handler

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/wei840222/simple-file-server/config"
	"golang.org/x/net/webdav"
)

const (
	style     = `<style>table {border-collapse: separate;border-spacing: 1.5em 0.25em;}h1 {padding-left: 0.3em;}a {text-decoration: none;color: blue;}.left {text-align: left;}.mono {font-family: monospace;}.mw20 {min-width: 20em;}</style>`
	meta      = `<meta name="referrer" content="no-referrer" />`
	listIndex = `<tr><th class="left mw20">Name</th><th class="left">Last modified</th><th>Size</th></tr><tr><th colspan="3"><hr></th></tr>`
	homeDIr   = "<tr><td><a href=\"%s\">Home Dir</a></td><td>&nbsp;</td><td class=\"mono\" align=\"right\">[DIR]</td></tr>"
	perDir    = `<td><a href="..">Pre Dir</a></td><td>&nbsp;</td><td class="mono" align="right">[DIR]</td></tr>`
	fileuri   = "<tr><td><a href=\"%s\" >%s</a></td><td class=\"mono\">%s</td><td class=\"mono\" align=\"right\">%s</td></tr>"
)

func path2index(path string) string {
	s := strings.Split(path, "/")
	var tmp string
	for k, v := range s[1 : len(s)-1] {
		tmp += fmt.Sprintf("/<a href = \"%s\">%s</a>", strings.Repeat("../", len(s)-3-k), v)
	}
	return tmp
}

func getsize(size int64) string {
	tmp := float64(size)
	if tmp < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if tmp = tmp / 1024; tmp < 1024 {
		return fmt.Sprintf("%.2f KB", tmp)
	} else if tmp = tmp / 1024; tmp < 1024 {
		return fmt.Sprintf("%.2f MB", tmp)
	} else if tmp = tmp / 1024; tmp < 1024 {
		return fmt.Sprintf("%.2f GB", tmp)
	} else {
		return fmt.Sprintf("%.2f TB", tmp/1024)
	}
}

type SortFileInfo []fs.FileInfo

func (x SortFileInfo) Len() int           { return len(x) }
func (x SortFileInfo) Less(i, j int) bool { return x[i].Name() < x[j].Name() }
func (x SortFileInfo) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type WebdavHandler struct {
	logger zerolog.Logger
	fs     webdav.Handler
}

func (h *WebdavHandler) generateWeb(FSInfo []fs.FileInfo, path string, writer io.Writer) {
	fmt.Fprintf(writer, "<html><head>%s<title>Index of %s</title>%s</head>", meta, path, style)
	fmt.Fprintf(writer, "<body><h1>Index of /<a href=\"%s\">Home</a>%s</h1><table>%s%s", "/webdav", path2index(path), listIndex, fmt.Sprintf(homeDIr, "/webdav"))
	if path != "/" {
		fmt.Fprint(writer, perDir)
	}
	var dirs = []fs.FileInfo{}
	var files = []fs.FileInfo{}
	for _, d := range FSInfo {
		if d.IsDir() {
			dirs = append(dirs, d)
		} else {
			files = append(files, d)
		}
	}
	sort.Sort(SortFileInfo(dirs))
	sort.Sort(SortFileInfo(files))
	for _, v := range dirs {
		name := v.Name() + "/"
		fmt.Fprintf(writer, fileuri, name, name, v.ModTime().Format("2006/1/2 15:04:05"), "[DIR]")
	}
	for _, v := range files {
		name := v.Name()
		fmt.Fprintf(writer, fileuri, name, name, v.ModTime().Format("2006/1/2 15:04:05"), getsize(v.Size()))

	}
	fmt.Fprint(writer, `</table></body></html>`)
}

func (h *WebdavHandler) handleDirList(fs webdav.FileSystem, c *gin.Context) bool {
	filePath := c.Params.ByName("webdav")
	f, err := fs.OpenFile(c, filePath, os.O_RDONLY, 0)
	if err != nil {
		h.logger.Warn().Err(err).Str("path", filePath).Msg("Failed to open file for directory listing")
		return false
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || fi == nil || !fi.IsDir() {
		return false
	}
	dirs, err := f.Readdir(-1)
	if err != nil {
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return true
	}
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.generateWeb(dirs, filePath, c.Writer)
	return true
}

func (h *WebdavHandler) HandlerRequest(c *gin.Context) {
	h.logger.Debug().Msg("WebDAV request received")
	if c.Request.Method == http.MethodGet && h.handleDirList(h.fs.FileSystem, c) {
		return
	}
	h.fs.ServeHTTP(c.Writer, c.Request)
}

func RegisterWebdavHandler(e *gin.Engine) {
	h := WebdavHandler{
		logger: log.With().Str("logger", "webdavHandler").Logger(),
		fs: webdav.Handler{
			Prefix:     "/webdav",
			FileSystem: webdav.Dir(viper.GetString(config.KeyFileRoot)),
			LockSystem: webdav.NewMemLS(),
		},
	}

	webdav := e.Group("/webdav")
	{
		webdav.Any("/*webdav", h.HandlerRequest)
		for _, method := range []string{"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK"} {
			webdav.Handle(method, "/*webdav", h.HandlerRequest)
		}
	}
}
