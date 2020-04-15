package server

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const sqlCreateSchema string = `
CREATE TABLE kv(key TEXT NOT NULL PRIMARY KEY, value BLOB);
CREATE TABLE users(uuid BLOB(16) NOT NULL PRIMARY KEY, cert BLOB NOT NULL, verified BOOL);
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
	addUser, retrieveUser, verifyUser *sql.Stmt
}

func prepareSQLStatements(db *sql.DB) (sqlStmts, error) {
	var (
		s   sqlStmts
		err error
	)

	s.addUser, err = db.Prepare("INSERT INTO users(uuid, cert, verified) VALUES(?, ?, false)")
	if err != nil {
		return s, fmt.Errorf("failed to prepare user creation statement: %w", err)
	}

	s.retrieveUser, err = db.Prepare("SELECT * FROM users WHERE uuid = ?")
	if err != nil {
		return s, fmt.Errorf("failed to prepare user retrieval statement: %w", err)
	}

	s.verifyUser, err = db.Prepare("UPDATE users SET verified = true WHERE uuid = ?")
	if err != nil {
		return s, fmt.Errorf("failed to prepare user verification statement: %w", err)
	}

	return s, nil
}

// User represents an API user
type User struct {
	UUID     uuid.UUID
	Cert     *x509.Certificate
	Verified bool
}

func (u User) uuidBytes() []byte {
	return u.UUID[:]
}

func (s *Server) getUser(uuidStr string) (User, error) {
	var u User

	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return u, fmt.Errorf("failed to parse UUID: %w", err)
	}

	var certDER []byte
	row := s.stmts.retrieveUser.QueryRow([]byte(id[:]))
	if err := row.Scan(&u.UUID, &certDER, &u.Verified); err != nil {
		return u, fmt.Errorf("failed to retrieve user from database: %w", err)
	}

	u.Cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return u, fmt.Errorf("failed to parse stored certificate for user: %w", err)
	}
	return u, nil
}

func (s *Server) userForCert(certDER []byte) (User, error) {
	var (
		u   User
		err error
	)

	u.Cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return u, fmt.Errorf("failed to parse certificate: %w", err)
	}

	u.UUID, err = uuid.Parse(u.Cert.Subject.CommonName)
	if err != nil {
		return u, fmt.Errorf("failed to parse cert UUID: %w", err)
	}

	var (
		storedUUID    uuid.UUID
		storedCertDER []byte
	)
	row := s.stmts.retrieveUser.QueryRow(u.uuidBytes())
	if err := row.Scan(&storedUUID, &storedCertDER, &u.Verified); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return u, fmt.Errorf("failed to retrieve user from database: %w", err)
	}

	if storedCertDER == nil {
		log.WithField("uuid", u.UUID.String()).Debug("Inserting new (unverified) user into DB")
		if _, err := s.stmts.addUser.Exec(u.uuidBytes(), u.Cert.Raw); err != nil {
			return u, fmt.Errorf("failed to insert new user into database: %w", err)
		}
	} else {
		storedCert, err := x509.ParseCertificate(storedCertDER)
		if err != nil {
			return u, fmt.Errorf("failed to parse stored user certificate: %w", err)
		}

		cp := x509.NewCertPool()
		cp.AddCert(storedCert)
		if _, err := u.Cert.Verify(x509.VerifyOptions{
			Roots: cp,
		}); err != nil {
			return u, fmt.Errorf("failed to verify user cert against stored cert: %w", err)
		}
	}

	return u, nil
}

func (s *Server) markUserVerified(u *User) error {
	if _, err := s.stmts.verifyUser.Exec(u.uuidBytes()); err != nil {
		return err
	}

	u.Verified = true
	return nil
}
