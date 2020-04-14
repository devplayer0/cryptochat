package server

import (
	"net/http"
	"path"
	"strings"

	"github.com/devplayer0/cryptochat/internal/data"
)

type spaHandler struct {
	fs    http.Handler
	inner http.Handler
}

func newSPAHandler() spaHandler {
	h := spaHandler{
		fs: http.FileServer(data.AssetFile()),
	}
	h.inner = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := data.Asset(r.URL.Path); err != nil {
			// file does not exist, serve index.html
			if _, err := w.Write(data.MustAsset("index.html")); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// otherwise, use http.FileServer to serve the static dir
		h.fs.ServeHTTP(w, r)
	})

	return h
}
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
	}
	upath = path.Clean(upath)
	r.URL.Path = upath

	if r.URL.Path == "/" {
		if _, err := w.Write(data.MustAsset("index.html")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	handler := h.inner
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		handler = http.StripPrefix("/assets/", handler)
	}
	handler.ServeHTTP(w, r)
}
