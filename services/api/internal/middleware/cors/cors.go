package cors

import (
	"net/http"

	"github.com/go-chi/cors"
)

// Cors returns CORS middleware for dashboard-facing routes with restricted origins.
// Allows GET, POST, PUT, DELETE, PATCH for authenticated mutation endpoints.
func Cors(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"https://cloud.zagforge.com"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
