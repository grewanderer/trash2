package pki

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"wisp/internal/models"
	"wisp/internal/repo"
)

type Service struct {
	Store *repo.PKIStore
	Now   func() time.Time
}

func New(store *repo.PKIStore) *Service { return &Service{Store: store, Now: time.Now} }

func (s *Service) EnsureRootCA(ctx context.Context, name string, ttl time.Duration) (*models.CA, error) {
	return s.Store.GetOrCreateCA(ctx, name, func() (*models.CA, error) {
		nb, na := s.Now().Add(-time.Hour), s.Now().Add(ttl)
		sk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		tpl := &x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{CommonName: name},
			NotBefore:    nb, NotAfter: na,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			BasicConstraintsValid: true, IsCA: true, MaxPathLenZero: true,
		}
		der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &sk.PublicKey, sk)
		var certPEM, keyPEM bytes.Buffer
		_ = pem.Encode(&certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: der})
		derKey, _ := x509.MarshalECPrivateKey(sk)
		_ = pem.Encode(&keyPEM, &pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey})
		return &models.CA{Name: name, CertPEM: certPEM.Bytes(), KeyPEM: keyPEM.Bytes(), NotBefore: nb, NotAfter: na}, nil
	})
}

func (s *Service) IssueDeviceCert(ctx context.Context, ca *models.CA, cn string, ttl time.Duration, deviceID *uint) (*models.Certificate, error) {
	nb, na := s.Now().Add(-time.Hour), s.Now().Add(ttl)
	sk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pb, _ := pem.Decode(ca.CertPEM)
	parent, _ := x509.ParseCertificate(pb.Bytes)
	kb, _ := pem.Decode(ca.KeyPEM)
	cakey, _ := x509.ParseECPrivateKey(kb.Bytes)
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    nb, NotAfter: na,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, parent, &sk.PublicKey, cakey)
	var certPEM, keyPEM bytes.Buffer
	_ = pem.Encode(&certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	derKey, _ := x509.MarshalECPrivateKey(sk)
	_ = pem.Encode(&keyPEM, &pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey})
	c := &models.Certificate{CAID: ca.ID, DeviceID: deviceID, CN: cn, CertPEM: certPEM.Bytes(), KeyPEM: keyPEM.Bytes(), NotBefore: nb, NotAfter: na}
	return c, s.Store.SaveCert(ctx, c)
}
