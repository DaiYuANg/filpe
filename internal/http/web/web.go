// Package web embeds HTTP templates and static web assets.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html
var files embed.FS

func TemplateFileSystem() http.FileSystem {
	templates, err := fs.Sub(files, "templates")
	if err != nil {
		return http.FS(files)
	}
	return http.FS(templates)
}
