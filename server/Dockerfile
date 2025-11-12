FROM --platform=$BUILDPLATFORM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o logdeck-server ./cmd/server

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/logdeck-server ./logdeck-server

ENTRYPOINT ["/app/logdeck-server"]
