package server

import (
	"fmt"
	"path"
	"strings"
)

func JoinURL(base string, paths ...string) string {
	filteredPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" || strings.TrimSpace(p) == "." || strings.TrimSpace(p) == ".." || strings.TrimSpace(p) == "/" || strings.TrimSpace(p) == "./" || strings.TrimSpace(p) == "../" {
			continue
		}
		filteredPaths = append(filteredPaths, p)
	}
	p := path.Join(filteredPaths...)
	p = fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
	p = strings.TrimPrefix(p, ".")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
}
