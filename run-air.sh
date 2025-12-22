#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
    echo "✅ Loaded environment from .env"
else
    echo "❌ No .env file found"
    exit 1
fi

# Run air with environment variables
echo "🚀 Starting air with DATABASE_URL=$DATABASE_URL"
exec air
