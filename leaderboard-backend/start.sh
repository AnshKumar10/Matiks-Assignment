#!/bin/bash
set -e

DATA_DIR=/var/lib/postgresql/data

# Initialize Postgres if empty
if [ ! -f "$DATA_DIR/PG_VERSION" ]; then
    echo "Initializing Postgres database..."
    su - postgres -c "initdb -D $DATA_DIR -A trust"
fi

# Start Postgres in the background
echo "Starting Postgres..."
su - postgres -c "postgres -D $DATA_DIR &"

# Wait for Postgres to be ready
echo "Waiting for Postgres..."
until su - postgres -c "pg_isready -d postgres" >/dev/null 2>&1; do
    sleep 1
done
echo "Postgres is ready."

# Create leaderboard database if it doesn't exist
echo "Ensuring 'leaderboard' database exists..."
su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='leaderboard'\" | grep -q 1 || psql -c 'CREATE DATABASE leaderboard;'"

# Start Redis in background
echo "Starting Redis..."
redis-server &
# Wait for Redis to be ready
echo "Waiting for Redis..."
until redis-cli ping >/dev/null 2>&1; do
    sleep 1
done
echo "Redis is ready."

# Run seed binary to populate database and rebuild Redis leaderboard
echo "Seeding the leaderboard database..."
./seed

# Start API and Worker
echo "Starting API..."
./api &

# Keep container alive
wait -n
