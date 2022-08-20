[![Build Status](https://github.com/azazeal/singleflight/actions/workflows/build.yml/badge.svg)](https://github.com/azazeal/singleflight/actions/workflows/build.yml)
[![Coverage Report](https://coveralls.io/repos/github/azazeal/singleflight/badge.svg?branch=master)](https://coveralls.io/github/azazeal/singleflight?branch=master)
[![Go Reference](https://pkg.go.dev/badge/github.com/azazeal/singleflight.svg)](https://pkg.go.dev/github.com/azazeal/singleflight)

# singleflight

Package singleflight implements a call sharing mechanism.

## Example usage

```go
// This package demonstrates how an implementation of a HTTPS server that uses self-signed 
// certificates might use the singleflight package. This code is only for demonstration purposes
// and may not be safe for production use.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/azazeal/singleflight"
)

var (
	cached cache // cached maintains a list of generated certificates (per server name)

	// caller allows us to be generating a single certificate per server name at a time
	caller singleflight.Caller[string, *tls.Certificate]
)

func main() {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second<<3)
			defer cancel()

			// only one invokation to fetchCertificate may be active at any given time 
			// per chi.ServerName since we're going through caller.
			return caller.Call(ctx, chi.ServerName, fetchCertificate)
		},
	}

	l, err := tls.Listen("tcp", ":4443", cfg)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", index)
	if err := http.Serve(l, nil); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "Howdie!\n")
}

func fetchCertificate(ctx context.Context) (*tls.Certificate, error) {
	serverName := caller.KeyFromContext(ctx)

	if cert := cached.get(serverName); cert != nil {
		return cert, nil
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048 /*derp*/)
	if err != nil {
		return nil, fmt.Errorf("failed generating private key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"AwesomeCorp"},
		},
		DNSNames:              []string{serverName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed creating certificate: %w", err)
	}

	certPem := pemEncode("CERTIFICATE", der)
	keyPEM := pemEncode("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))

	cert, err := tls.X509KeyPair(certPem, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed loading X509 keypair: %w", err)
	}
	cached.put(serverName, &cert)

	return &cert, nil
}

func pemEncode(typ string, data []byte) []byte {
	var b bytes.Buffer
	if err := pem.Encode(&b, &pem.Block{Type: typ, Bytes: data}); err != nil {
		panic(err)
	}
	return append([]byte(nil), b.Bytes()...)
}

type cache struct {
	mu    sync.Mutex
	certs map[string]*tls.Certificate
}

func (c *cache) get(serverName string) *tls.Certificate {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.certs[serverName]
}

func (c *cache) put(serverName string, cert *tls.Certificate) {
	fingerprint := sha256.Sum256(cert.Certificate[0])
	defer log.Printf("generated cert for %s: %x", serverName, fingerprint)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.certs == nil {
		c.certs = make(map[string]*tls.Certificate)
	}
	c.certs[serverName] = cert
}
```
