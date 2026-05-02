FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/CosmoLabs-org/cosmo-smoke/cmd.Version=$(git describe --tags --always 2>/dev/null || echo dev)" -o /smoke .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /smoke /usr/local/bin/smoke
WORKDIR /app
ENTRYPOINT ["smoke"]
CMD ["run"]
