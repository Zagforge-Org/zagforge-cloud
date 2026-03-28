package token

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	errDecodePrivateKey     = errors.New("decode private key")
	errParsePrivateKey      = errors.New("parse private key")
	errDecodePublicKey      = errors.New("decode public key")
	errParsePublicKey       = errors.New("parse public key")
	errPrivateKeyNotEd25519 = errors.New("private key is not Ed25519")
	errPublicKeyNotEd25519  = errors.New("public key is not Ed25519")
)

// Service handles JWT signing and refresh token generation.
type Service struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// New creates a token service from base64-encoded Ed25519 keys.
func New(privKeyB64, pubKeyB64, issuer string, accessTTL, refreshTTL time.Duration) (*Service, error) {
	privPEM, err := base64.StdEncoding.DecodeString(privKeyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errDecodePrivateKey, err)
	}

	privKey, err := jwt.ParseEdPrivateKeyFromPEM(privPEM)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errParsePrivateKey, err)
	}

	pubPEM, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errDecodePublicKey, err)
	}

	pubKey, err := jwt.ParseEdPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errParsePublicKey, err)
	}

	edPriv, ok := privKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errPrivateKeyNotEd25519
	}
	edPub, ok := pubKey.(ed25519.PublicKey)
	if !ok {
		return nil, errPublicKeyNotEd25519
	}

	return &Service{
		privateKey: edPriv,
		publicKey:  edPub,
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

// PublicKey returns the Ed25519 public key for JWKS.
func (s *Service) PublicKey() ed25519.PublicKey {
	return s.publicKey
}

// Issuer returns the configured issuer.
func (s *Service) Issuer() string {
	return s.issuer
}

// AccessTokenTTL returns the configured access token TTL.
func (s *Service) AccessTokenTTL() time.Duration {
	return s.accessTTL
}

// RefreshTokenTTL returns the configured refresh token TTL.
func (s *Service) RefreshTokenTTL() time.Duration {
	return s.refreshTTL
}
