FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=$(git describe --tags --always 2>/dev/null || echo dev)" -o /smokesig .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -h /app smokesig
COPY --from=builder /smokesig /usr/local/bin/smokesig
USER smokesig
WORKDIR /app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD ["/usr/local/bin/smokesig", "version"]
STOPSIGNAL SIGTERM
ENTRYPOINT ["smokesig"]
CMD ["run"]
