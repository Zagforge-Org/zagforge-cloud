package oauth

import (
	"net/http"

	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// JWKSKey represents a single key in a JWKS response.
type JWKSKey struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	Use       string `json:"use"`
	KeyID     string `json:"kid"`
	PublicKey string `json:"x"`
}

// JWKSResponse represents the JSON Web Key Set response.
type JWKSResponse struct {
	Keys []JWKSKey `json:"keys"`
}

// JWKS returns the public key in JWK format.
func (h *Handler) JWKS(w http.ResponseWriter, _ *http.Request) {
	resp := JWKSResponse{
		Keys: []JWKSKey{
			{
				KeyType:   "OKP",
				Curve:     "Ed25519",
				Use:       "sig",
				KeyID:     h.jwksKID,
				PublicKey: httputil.Base64URLEncode(h.tokenSvc.PublicKey()),
			},
		},
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}
