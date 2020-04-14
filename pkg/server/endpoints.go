package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

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
