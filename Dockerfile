FROM golang:1.27.1-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/ozmap-user-manager ./cmd/server
FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=builder /out/ozmap-user-manager /app/ozmap-user-manager
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/ozmap-user-manager"]
