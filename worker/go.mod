module github.com/LegationPro/zagforge-mvp-impl/worker

go 1.26.1

require (
	github.com/LegationPro/zagforge-mvp-impl/api v0.0.0
	github.com/LegationPro/zagforge-mvp-impl/shared/go v0.0.0
	github.com/jackc/pgx/v5 v5.8.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace (
	github.com/LegationPro/zagforge-mvp-impl/api => ../api
	github.com/LegationPro/zagforge-mvp-impl/shared/go => ../shared/go
)
