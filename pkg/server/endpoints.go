package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// ParseJSONBody attempts to parse the request body as JSON
func ParseJSONBody(v interface{}, w http.ResponseWriter, r *http.Request) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		JSONErrResponse(w, fmt.Errorf("failed to parse request body: %w", err), http.StatusBadRequest)
		return err
	}

	return nil
}

// JSONResponse Sends a JSON payload in response to a HTTP request
func JSONResponse(w http.ResponseWriter, v interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		log.WithField("err", err).Error("Failed to serialize JSON payload")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Failed to serialize JSON payload")
	}
}

type jsonError struct {
	Message string `json:"message"`
}

// JSONErrResponse Sends an `error` as a JSON object with a `message` property
func JSONErrResponse(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)

	enc := json.NewEncoder(w)
	enc.Encode(jsonError{err.Error()})
}

// JSONReq attempts to make a POST request in JSON and decode a response
func JSONReq(c *http.Client, method, url string, b interface{}, r interface{}) error {
	body, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer res.Body.Close()

	d := json.NewDecoder(res.Body)
	if res.StatusCode >= 400 {
		var e jsonError
		if err := d.Decode(&e); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %w", err)
		}

		return fmt.Errorf("server responded with HTTP %v %v", res.StatusCode, e.Message)
	}

	if r == nil {
		return nil
	}
	if err := d.Decode(r); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return nil
}

type apiReqSendMessage struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func (s *Server) apiSendMessage(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(keyUser).(User)

	var b apiReqSendMessage
	if err := ParseJSONBody(&b, w, r); err != nil {
		return
	}

	vars := mux.Vars(r)
	room := vars["room"]
	if !s.discovery.IsMember(room) {
		JSONErrResponse(w, errors.New("user is not a member of this room"), http.StatusBadRequest)
		return
	}

	s.publishJSON(streamMessages, uiEventMessage{
		Sender: uiMessageSender{
			Username: b.Username,
			UUID:     u.UUID.String(),
		},
		Room:    vars["room"],
		Content: b.Content,
	})
}
