# GoHypo Development Environment - Database Management
# Note: Use 'air' to run the Go application with live reload

.PHONY: help init-db db-up db-down db-logs db-reset db-admin db-status migrate test build clean dev css-build css-watch css-install

help: ## Show this help message
	@echo "GoHypo Database Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
	@echo ""
	@echo "Note: Use 'air' to run the Go application with live reload"

init-db: ## Initialize database with Docker (first-time setup)
	./scripts/init-db.sh

db-up: ## Start PostgreSQL database
	docker-compose up -d postgres

db-down: ## Stop PostgreSQL database
	docker-compose down

db-logs: ## Show database logs
	docker-compose logs -f postgres

db-reset: ## Reset database (WARNING: destroys all data)
	docker-compose down -v
	docker-compose up -d postgres

db-admin: ## Start pgAdmin for database management
	docker-compose up -d pgadmin

db-status: ## Show database container status
	docker-compose ps

migrate: ## Show migration status (migrations run automatically on app startup)
	@echo "Migrations are handled automatically by the application on startup"

test: ## Run tests
	go test ./...

build: ## Build the application
	go build -o bin/gohypo .

clean: ## Clean build artifacts
	rm -rf bin/
	go clean

dev: ## Start database for development (use 'air' separately to run the app)
	docker-compose up -d postgres
	@echo "Database started. Now run 'air' in another terminal to start the application."

css-install: ## Install CSS build dependencies (Node.js/npm required)
	npm install

css-build: ## Build CSS with Tailwind (requires npm install first)
	npm run build-css

css-watch: ## Watch CSS files and rebuild on changes (requires npm install first)
	npm run watch-css