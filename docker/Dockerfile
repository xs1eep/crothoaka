# ---- build ----
FROM golang:1.27.3-alpine3.24 AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/crothoaka ./cmd/main.go

# ---- runtime ----
FROM alpine:3.24
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=build /out/crothoaka /app/crothoaka
ENTRYPOINT ["/app/crothoaka"]
