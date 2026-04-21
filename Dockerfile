# Stage 1: build
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /manager ./cmd/manager

# Stage 2: runtime (distroless)
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /manager /manager

USER nonroot:nonroot
ENTRYPOINT ["/manager"]
