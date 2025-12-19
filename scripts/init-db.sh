#!/bin/bash

# GoHypo Database Initialization Script
# This script helps set up the database for development

set -e

echo "🚀 GoHypo Database Setup"
echo "========================"

# Check if docker-compose is available
if command -v docker-compose &> /dev/null; then
    echo "📦 Starting PostgreSQL with Docker Compose..."
    docker-compose up -d postgres

    echo "⏳ Waiting for database to be ready..."
    sleep 5

    # Wait for database to be healthy
    echo "🏥 Checking database health..."
    max_attempts=30
    attempt=1
    while [ $attempt -le $max_attempts ]; do
        if docker-compose exec -T postgres pg_isready -U gohypo_user -d gohypo &>/dev/null; then
            echo "✅ Database is ready!"
            break
        fi
        echo "Attempt $attempt/$max_attempts: Database not ready yet..."
        sleep 2
        ((attempt++))
    done

    if [ $attempt -gt $max_attempts ]; then
        echo "❌ Database failed to start within expected time"
        exit 1
    fi

    echo "📊 Database URL: postgres://gohypo_user:gohypo_password@localhost:5432/gohypo?sslmode=disable"
    echo ""
    echo "💡 Next steps:"
    echo "1. Copy env.example to .env"
    echo "2. Add your OPENAI_API_KEY to .env"
    echo "3. Run: make run"
    echo ""
    echo "🔧 Optional: Start pgAdmin with 'make db-admin' (http://localhost:5050)"

else
    echo "❌ Docker Compose not found. Please install Docker and Docker Compose."
    echo ""
    echo "Manual setup instructions:"
    echo "1. Install PostgreSQL"
    echo "2. Create database: gohypo"
    echo "3. Create user: gohypo_user with password: gohypo_password"
    echo "4. Set DATABASE_URL in your .env file"
    exit 1
fi