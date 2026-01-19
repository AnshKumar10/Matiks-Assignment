# Stage 1: Build Go binaries
FROM golang:1.26rc2-alpine3.22 AS build
WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy all source files
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o seed ./seed

# Stage 2: Final lightweight image
FROM alpine:3.19
WORKDIR /app

# Install Postgres + Redis + bash
RUN apk add --no-cache postgresql redis bash postgresql-contrib

# Prepare Postgres runtime directories
RUN mkdir -p /var/lib/postgresql/data /run/postgresql \
    && chown -R postgres:postgres /var/lib/postgresql/data /run/postgresql \
    && chmod 700 /var/lib/postgresql/data

# Copy Go binaries
COPY --from=build /app/api .
COPY --from=build /app/worker .
COPY --from=build /app/seed .

# Copy start script
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# Expose API port
EXPOSE 8080

# Default command
CMD ["/app/start.sh"]
