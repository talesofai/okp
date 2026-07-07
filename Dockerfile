FROM golang:1.26 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /bin/okp-api ./cmd/api && \
    CGO_ENABLED=1 go build -o /bin/okp ./cmd/cli

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/okp-api /bin/okp-api
COPY --from=builder /bin/okp /bin/okp
COPY skills/ /skills/

EXPOSE 8720
CMD ["/bin/okp-api"]
