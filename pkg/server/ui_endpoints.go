package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/devplayer0/cryptochat/internal/data"
	"github.com/gorilla/mux"
	"github.com/r3labs/sse"
	log "github.com/sirupsen/logrus"
)

const streamVerification = "verification"
const streamMessages = "messages"

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

func (s *Server) publishJSON(stream string, v interface{}) error {
	enc, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("Failed to encode message payload: %w", err)
	}

	s.events.Publish(stream, &sse.Event{
		Data: enc,
	})
	return nil
}

type uiEventMessage struct {
	Room    string `json:"room"`
	Content string `json:"content"`
}

func (s *Server) uiVerifyUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	u, err := s.getUser(vars["uuid"])
	if err != nil {
		JSONErrResponse(w, fmt.Errorf("failed to get user: %w", err), http.StatusInternalServerError)
		return
	}

	if ch, ok := s.verification[u.UUID]; ok {
		if err := s.markUserVerified(&u); err != nil {
			JSONErrResponse(w, fmt.Errorf("failed"), http.StatusInternalServerError)
			return
		}
		delete(s.verification, u.UUID)
		close(ch)

		log.WithField("uuid", vars["uuid"]).Info("Marked user as verified")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	JSONErrResponse(w, errors.New("user verification not in progress"), http.StatusBadRequest)
}

func (s *Server) uiSendMessage(w http.ResponseWriter, r *http.Request) {
	var req apiReqSendMessage
	if err := ParseJSONBody(&req, w, r); err != nil {
		return
	}

	vars := mux.Vars(r)
	if err := JSONReq(s.client, http.MethodPost, fmt.Sprintf("https://%v/rooms/%v/message", s.peerAddr, vars["room"]),
		apiReqSendMessage{req.Content}, nil); err != nil {
		JSONErrResponse(w, err, http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusNoContent)
}
