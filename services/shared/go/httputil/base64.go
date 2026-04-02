package httputil

import "encoding/base64"

// Base64URLEncode returns the unpadded base64url encoding of data.
func Base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
