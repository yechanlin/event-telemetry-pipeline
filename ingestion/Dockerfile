# --- build stage: compile the Go binary ---
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o ingestion .

# --- run stage: tiny image with just the binary ---
FROM alpine:latest
WORKDIR /app
COPY --from=build /app/ingestion .
EXPOSE 9000
CMD ["./ingestion"]
