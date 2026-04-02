FROM golang:1.26.1-alpine AS build
RUN apk add --no-cache git

WORKDIR /app
COPY go.work go.work.sum ./
COPY services/api/go.mod services/api/go.sum ./services/api/
COPY services/auth/go.mod services/auth/go.sum ./services/auth/
COPY services/worker/go.mod services/worker/go.su[m] ./services/worker/
COPY services/shared/go/go.mod services/shared/go/go.sum ./services/shared/go/
RUN go work sync
COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/server ./services/api/cmd/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/server /bin/server
EXPOSE 8080
ENTRYPOINT ["/bin/server"]
