package server

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	// SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

const rsaBits int = 2048
const certValidity = 365 * 24 * time.Hour

// Server is a CryptoChat server
type Server struct {
	db *sql.DB

	cert *tls.Certificate
	http http.Server
}

// NewServer creates a new Server
func NewServer(dbPath string) (*Server, error) {
	dbInitRequired := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dbInitRequired = true
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	s := Server{
		db: db,
	}

	if dbInitRequired {
		if err := s.dbInit(); err != nil {
			return nil, err
		}
	}

	cert, err := s.loadCert()
	s.http = http.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	s.cert = &s.http.TLSConfig.Certificates[0]

	return &s, nil
}

// Listen begins listening
func (s *Server) Listen(addr string) error {
	s.http.Addr = addr
	if err := s.http.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Close ends listening
func (s *Server) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	if err := s.http.Close(); err != nil {
		return fmt.Errorf("failed to close http server: %w", err)
	}
	return nil
}
