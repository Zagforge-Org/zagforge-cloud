module github.com/LegationPro/zagforge/auth

go 1.26.1

require (
	github.com/LegationPro/zagforge/shared/go v0.0.0
	github.com/caarlos0/env/v11 v11.4.0
	github.com/go-chi/chi/v5 v5.2.5
	github.com/go-chi/docgen v1.3.0
	github.com/go-playground/validator/v10 v10.30.1
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.8.0
	github.com/joho/godotenv v1.5.1
	github.com/pquerna/otp v1.5.0
	github.com/redis/go-redis/v9 v9.18.0
	go.uber.org/zap v1.27.1
	golang.org/x/crypto v0.49.0
	golang.org/x/oauth2 v0.36.0
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/go-chi/cors v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace github.com/LegationPro/zagforge/shared/go => ../shared/go
