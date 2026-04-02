package contexturl

import handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"

func tokenHash(raw string) string {
	return handlerpkg.SHA256Hash(raw)
}
