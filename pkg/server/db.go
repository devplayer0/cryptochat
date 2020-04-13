package server

import (
	"crypto/tls"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const sqlCreateSchema string = `
CREATE TABLE kv(key TEXT NOT NULL PRIMARY KEY, value BLOB);
CREATE TABLE users(uuid TEXT NOT NULL PRIMARY KEY, cert BLOB NOT NULL, name TEXT);
`

func (s *Server) dbInit() error {
	if _, err := s.db.Exec(sqlCreateSchema); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Infof("Generating %v bit RSA key and certificate", rsaBits)
	cert, err := GenerateCert(rsaBits, uuid.New().String(), certValidity)
	if err != nil {
		return fmt.Errorf("failed to generate cert and key: %w", err)
	}

	stmt, err := s.db.Prepare("INSERT INTO kv(key, value) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement: %w", err)
	}
	defer stmt.Close()

	certDER, privDER := GetCertDER(&cert)
	if _, err := stmt.Exec("cert", certDER); err != nil {
		return fmt.Errorf("failed to insert cert into database: %w", err)
	}
	if _, err := stmt.Exec("key", privDER); err != nil {
		return fmt.Errorf("failed to insert key into database: %w", err)
	}

	return nil
}

func (s *Server) loadCert() (tls.Certificate, error) {
	var c tls.Certificate
	rows, err := s.db.Query("SELECT key, value FROM kv WHERE key IN ('cert', 'key')")
	if err != nil {
		return c, fmt.Errorf("failed to query database for cert and key: %w", err)
	}
	defer rows.Close()

	var certDER, keyDER []byte
	for rows.Next() {
		var (
			key   string
			value []byte
		)
		if err := rows.Scan(&key, &value); err != nil {
			return c, fmt.Errorf("failed to read row from query result: %w", err)
		}

		if key == "cert" {
			certDER = value
		} else if key == "key" {
			keyDER = value
		}
	}

	if certDER == nil || keyDER == nil {
		return c, fmt.Errorf("failed to find cert and key in database: %w", err)
	}
	return LoadCert(certDER, keyDER)
}

type sqlStmts struct {
}
