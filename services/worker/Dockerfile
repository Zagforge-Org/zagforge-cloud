FROM golang:1.26.1-alpine AS build
RUN apk add --no-cache git curl tar gcompat

# Install zigzag from prebuilt release
ARG TARGETARCH
ARG ZIGZAG_VERSION=v0.18.0
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) ZIGZAG_ARCH="x86_64" ;; \
      arm64) ZIGZAG_ARCH="aarch64" ;; \
      *) echo "unsupported arch: ${TARGETARCH}" && exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/LegationPro/zigzag/releases/download/${ZIGZAG_VERSION}/zigzag-${ZIGZAG_ARCH}-linux.tar.gz" \
      | tar xz -C /usr/local/bin

WORKDIR /app
COPY go.work go.work.sum ./
COPY services/api/go.mod services/api/go.sum ./services/api/
COPY services/auth/go.mod services/auth/go.sum ./services/auth/
COPY services/worker/go.mod services/worker/go.su[m] ./services/worker/
COPY services/shared/go/go.mod services/shared/go/go.sum ./services/shared/go/
RUN go work sync
COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/worker ./services/worker/cmd/

FROM alpine:3.21
RUN apk add --no-cache git gcompat ca-certificates && \
    mkdir -p /data/reports && chown nobody:nobody /data/reports
COPY --from=build /usr/local/bin/zigzag /usr/local/bin/zigzag
COPY --from=build /bin/worker /bin/worker
USER nobody
ENTRYPOINT ["/bin/worker"]
