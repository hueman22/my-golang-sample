# Build stage
FROM golang:1.22 AS builder
WORKDIR /src/app
COPY app/go.mod app/go.sum* ./
RUN go mod download
COPY app/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app

# Run stage (distroless, không có shell)
FROM gcr.io/distroless/base-debian12
ENV APP_PORT=8090
EXPOSE 8090
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
