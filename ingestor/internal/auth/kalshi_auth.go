package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

type KalshiAuth struct {
	KeyID      string
	PrivateKey *rsa.PrivateKey
}

func NewKalshiAuth(keyID, pemPath string) (*KalshiAuth, error) {
	keyData, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS8 first (standard), then PKCS1
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
	}

	return &KalshiAuth{
		KeyID:      keyID,
		PrivateKey: priv.(*rsa.PrivateKey),
	}, nil
}

// GenerateHeaders creates the specific headers Kalshi requires
func (a *KalshiAuth) GenerateHeaders(method, path string) (http.Header, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	msg := timestamp + method + path

	// SHA256 Hash
	hashed := sha256.Sum256([]byte(msg))

	// RSA-PSS Sign
	signature, err := rsa.SignPSS(
		rand.Reader,
		a.PrivateKey,
		crypto.SHA256,
		hashed[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash},
	)
	if err != nil {
		return nil, err
	}

	sigBase64 := base64.StdEncoding.EncodeToString(signature)

	headers := http.Header{}
	headers.Set("KALSHI-ACCESS-KEY", a.KeyID)
	headers.Set("KALSHI-ACCESS-SIGNATURE", sigBase64)
	headers.Set("KALSHI-ACCESS-TIMESTAMP", timestamp)
	return headers, nil
}

// GetWebSocketHeaders generates headers for WebSocket connections
func (a *KalshiAuth) GetWebSocketHeaders() (http.Header, error) {
	// WebSocket upgrade uses GET method
	return a.GenerateHeaders("GET", "/trade-api/ws/v2")
}