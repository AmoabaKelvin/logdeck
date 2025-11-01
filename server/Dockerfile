FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o logdeck-server ./cmd/server

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/logdeck-server ./logdeck-server

ENTRYPOINT ["/app/logdeck-server"]
