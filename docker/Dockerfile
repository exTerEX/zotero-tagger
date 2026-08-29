# Build stage
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -o /zotero-tagger ./cmd/zotero-tagger

# Runtime stage
FROM alpine:3.24

RUN apk add --no-cache ca-certificates

COPY --from=builder /zotero-tagger /usr/local/bin/zotero-tagger
COPY config/config.toml /etc/zotero-tagger/config.toml

ENTRYPOINT ["zotero-tagger"]
CMD ["tag", "--config", "/etc/zotero-tagger/config.toml"]
