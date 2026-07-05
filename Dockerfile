# syntax=docker/dockerfile:1

# Pure-Go dynamic-PIE build (no CGO/glibc), for the x86_64 metro. Unikraft's ELF
# loader needs a PIE; the resulting dynamic PIE uses the musl loader provided by
# the alpine final stage. CA certs are required for the TLS connection to the
# Postgres FQDN.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildmode=pie -ldflags "-s -w" -o /api ./cmd/api

FROM --platform=linux/amd64 alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /api /api
ENTRYPOINT ["/api"]
