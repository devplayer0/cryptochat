package server

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/r3labs/sse"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const rsaBits int = 2048
const certValidity = 365 * 24 * time.Hour

type key int

const (
	keyServer key = iota
	keyUser
)

func writeAccessLog(t string) func(w io.Writer, params handlers.LogFormatterParams) {
	return func(w io.Writer, params handlers.LogFormatterParams) {
		log.WithFields(log.Fields{
			"remote":  params.Request.RemoteAddr,
			"agent":   params.Request.UserAgent(),
			"status":  params.StatusCode,
			"resSize": params.Size,
		}).Debugf("%v %v %v", t, params.Request.Method, params.URL.RequestURI())
	}
}

func userMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := r.Context().Value(keyServer).(*Server)

		u, err := s.getUser(r.TLS.PeerCertificates[0].Subject.CommonName)
		if err != nil {
			JSONErrResponse(w, fmt.Errorf("failed to internally retrieve connected user from TLS state: %w", err),
				http.StatusInternalServerError)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), keyUser, u))
		next.ServeHTTP(w, r)
	})
}

// Server is a CryptoChat server
type Server struct {
	db    *sql.DB
	stmts sqlStmts

	cert *tls.Certificate
	api  http.Server

	ui     http.Server
	events *sse.Server

	verificationLock sync.RWMutex
	verification     map[uuid.UUID]chan struct{}

	peerAddr string
	client   *http.Client
}

// NewServer creates a new Server
func NewServer(dbPath, peerAddr string) (*Server, error) {
	oldMask := -1
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		oldMask = unix.Umask(0066)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	s := Server{
		db:       db,
		peerAddr: peerAddr,

		verification: make(map[uuid.UUID]chan struct{}),
	}

	if oldMask != -1 {
		if err := s.dbInit(); err != nil {
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}
		unix.Umask(oldMask)
	}

	s.stmts, err = prepareSQLStatements(db)
	if err != nil {
		return nil, err
	}

	cert, err := s.loadCert()
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}
	log.WithField("fingerprint", GetCertFingerprint(cert.Leaf)).Info("Loaded server certificate")

	apiRouter := mux.NewRouter()
	apiRouter.Use(userMiddleware)
	apiRouter.HandleFunc("/rooms/{room}/message", s.apiSendMessage).Methods(http.MethodPost)

	s.api = http.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},

			ClientAuth:            tls.RequestClientCert,
			VerifyPeerCertificate: s.verifyPeer,
		},
		BaseContext: func(_ net.Listener) context.Context {
			return context.WithValue(context.Background(), keyServer, &s)
		},
		Handler: handlers.CustomLoggingHandler(nil, apiRouter, writeAccessLog("api")),
	}
	s.cert = &s.api.TLSConfig.Certificates[0]

	uiRouter := mux.NewRouter()

	uiAPI := uiRouter.PathPrefix("/api").Subrouter()
	uiAPI.HandleFunc("/users/{uuid}/verify", s.uiVerifyUser).Methods(http.MethodPost)
	uiAPI.HandleFunc("/rooms/{room}/message", s.uiSendMessage).Methods(http.MethodPost)

	s.events = sse.New()
	s.events.CreateStream(streamVerification)
	s.events.CreateStream(streamMessages)
	uiAPI.HandleFunc("/events", s.events.HTTPHandler).Methods(http.MethodGet)

	uiRouter.PathPrefix("/").Handler(newSPAHandler())

	s.ui = http.Server{
		Handler: handlers.CustomLoggingHandler(nil, uiRouter, writeAccessLog("ui")),
	}

	s.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},

				InsecureSkipVerify:    true,
				VerifyPeerCertificate: s.verifyPeer,
			},
		},
	}

	return &s, nil
}

// Listen begins listening
func (s *Server) Listen(addr, uiAddr string) error {
	s.api.Addr = addr
	s.ui.Addr = uiAddr

	err := make(chan error)
	go func() {
		err <- s.api.ListenAndServeTLS("", "")
		s.api.Close()
	}()
	go func() {
		err <- s.ui.ListenAndServe()
		s.ui.Close()
	}()

	if err := <-err; err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Close ends listening
func (s *Server) Close() error {
	s.events.Close()
	if err := s.ui.Close(); err != nil {
		return fmt.Errorf("failed to close frontend server: %w", err)
	}
	if err := s.api.Close(); err != nil {
		return fmt.Errorf("failed to close api server: %w", err)
	}

	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}
