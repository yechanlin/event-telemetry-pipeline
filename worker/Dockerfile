# --- build stage: compile the Go binary ---
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o worker .

# --- run stage: tiny image with just the binary ---
FROM alpine:latest
WORKDIR /app
COPY --from=build /app/worker .
CMD ["./worker"]
