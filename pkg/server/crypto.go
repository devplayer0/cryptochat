package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	log "github.com/sirupsen/logrus"
)

// GenerateCert creates a TLS certificate and RSA private key
func GenerateCert(keyBits int, name string, validFor time.Duration) (tls.Certificate, error) {
	var c tls.Certificate
	priv, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return c, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	now := time.Now()

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return c, fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: name,
		},
		NotBefore: now,
		NotAfter:  now.Add(validFor),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	data, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return c, fmt.Errorf("failed to create x509 certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{data},
		PrivateKey:  priv,
	}, nil
}

// GetCertDER returns the DER-encoded certificate and private key from a tls.Certificate
func GetCertDER(cert *tls.Certificate) ([]byte, []byte) {
	var keyDER []byte = nil
	if cert.PrivateKey != nil {
		keyDER = x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey))
	}

	return cert.Certificate[0], keyDER
}

// LoadCert loads a tls.Certificate from a DER certificate and optional private key
func LoadCert(cert, key []byte) (tls.Certificate, error) {
	var c tls.Certificate
	var err error
	var priv *rsa.PrivateKey
	if key != nil {
		priv, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return c, fmt.Errorf("failed to parse PKCS1 private key: %w", err)
		}
	}

	x509Cert, err := x509.ParseCertificate(cert)
	if err != nil {
		return c, fmt.Errorf("failed to parse X.509 certificate from DER: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  priv,
		Leaf:        x509Cert,
	}, nil
}

// GetCertFingerprint gets an X.509 certificate's fingerprint
func GetCertFingerprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw)
	return hex.EncodeToString(sum[:])
}

type verificationInfo struct {
	UUID        string `json:"uuid"`
	Fingerprint string `json:"fingerprint"`
}

func (s *Server) verifyPeer(certs [][]byte, _ [][]*x509.Certificate) error {
	u, err := s.userForCert(certs[0])
	if err != nil {
		return err
	}

	if !u.Verified {
		log.WithField("uuid", u.UUID.String()).Debug("Waiting for user verification")
		s.verificationLock.RLock()
		_, ok := s.verification[u.UUID]
		s.verificationLock.RUnlock()
		if !ok {
			s.verificationLock.Lock()
			s.verification[u.UUID] = make(chan struct{}, 1)
			s.verificationLock.Unlock()

			s.publishJSON(streamVerification, verificationInfo{
				UUID:        u.UUID.String(),
				Fingerprint: GetCertFingerprint(u.Cert),
			})
		}

		s.verificationLock.RLock()
		ch := s.verification[u.UUID]
		s.verificationLock.RUnlock()

		<-ch
		u, err := s.userForCert(certs[0])
		if err != nil {
			return err
		}

		if !u.Verified {
			return errors.New("verification was rejected")
		}
	}

	log.WithField("uuid", u.UUID.String()).Debug("Peer verification passed")
	return nil
}
