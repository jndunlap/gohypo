# 🐳 GoHypo Docker Setup Guide

This guide explains how to run GoHypo with PostgreSQL using Docker Compose.

## 📋 Prerequisites

- Docker & Docker Compose
- Go 1.24+ (for local development)
- OpenAI API Key

## 🚀 Quick Start

### 1. Initialize Database
```bash
make init-db
# Or: ./scripts/init-db.sh
```

This will:
- Start PostgreSQL container
- Create the database schema automatically
- Wait for database readiness

### 2. Configure Environment
```bash
cp env.example .env
# Edit .env with your OpenAI API key
```

### 3. Run the Application
```bash
make run
# Or: go run main.go
```

### 4. Access Services
- **GoHypo UI**: http://localhost:8081
- **pgAdmin** (optional): http://localhost:5050
  - Email: `admin@gohypo.local`
  - Password: `admin123`

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   GoHypo UI     │────│  Service Layer   │────│  PostgreSQL     │
│   (Port 8081)   │    │                  │    │                 │
│                 │    │ - SessionManager │    │ - users         │
│ - Research UI   │    │ - ResearchWorker │    │ - sessions      │
└─────────────────┘    └──────────────────┘    │ - hypotheses    │
                                               └─────────────────┘
                                                ┌──────────────┐
                                                │   pgAdmin    │
                                                │  (Port 5050) │
                                                └──────────────┘
```

## 📁 Docker Compose Files

### `docker-compose.yml` - Base Configuration
- PostgreSQL 15 with Alpine Linux
- Automatic schema initialization
- Health checks and restart policies
- Optional pgAdmin for database management

### `docker-compose.override.yml` - Development Overrides
- Development database credentials
- Different port mapping (5433) to avoid conflicts
- Development pgAdmin credentials

### `docker-compose.prod.yml` - Production Configuration
- Environment variable-based configuration
- No exposed ports (security)
- pgAdmin only available in debug profile

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `OPENAI_API_KEY` | OpenAI API key for research | Required |
| `LLM_MODEL` | OpenAI model to use | `gpt-4-turbo-preview` |
| `PROMPTS_DIR` | Directory with research prompts | `./prompts` |
| `EXCEL_FILE` | Path to data file | `./final_dataset.csv` |
| `PORT` | Application port | `8081` |

### Database Connection

**Development:**
```
postgres://gohypo_dev:dev_password@localhost:5433/gohypo_dev?sslmode=disable
```

**Production:**
```
postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@$POSTGRES_HOST:$POSTGRES_PORT/$POSTGRES_DB?sslmode=require
```

## 🛠️ Development Commands

```bash
# Database Management
make init-db     # First-time database setup
make db-up       # Start database
make db-down     # Stop database
make db-reset    # Reset database (WARNING: destroys data!)
make db-admin    # Start pgAdmin
make db-logs     # View database logs
make db-status   # Show container status

# Application
make build       # Build Go binary
make run         # Run application
make dev         # Start everything (db + app)
make test        # Run tests
make clean       # Clean build artifacts
```

## 🔄 Database Schema

The database schema is automatically created on first run:

- **`users`**: User accounts (single user initially)
- **`research_sessions`**: Research session metadata and progress
- **`hypothesis_results`**: Validated research findings with JSONB storage

### Indexes
- User-scoped indexes on all tables
- JSONB GIN indexes for complex queries
- Composite indexes for common query patterns

## 🔒 Security Considerations

### Development
- Default passwords (change for production)
- Exposed database ports
- pgAdmin accessible

### Production
- Use `docker-compose.prod.yml`
- Set strong passwords via environment variables
- No exposed database ports
- pgAdmin only in debug mode

## 🐛 Troubleshooting

### Database Connection Issues
```bash
# Check if database is running
make db-status

# View database logs
make db-logs

# Reset database
make db-reset
```

### Application Won't Start
1. Verify `.env` file exists with correct `DATABASE_URL`
2. Check database is healthy: `make db-logs`
3. Verify OpenAI API key is set

### Port Conflicts
- Development uses port 5433 for PostgreSQL
- Production uses standard ports
- Check `docker-compose.override.yml` for port configuration

## 📊 Database Management

### Using pgAdmin
1. Start pgAdmin: `make db-admin`
2. Open http://localhost:5050
3. Login with development credentials
4. Add server connection:
   - Host: `postgres` (container name)
   - Database: `gohypo_dev`
   - Username: `gohypo_dev`
   - Password: `dev_password`

### Manual Database Access
```bash
# Connect to database container
docker-compose exec postgres psql -U gohypo_dev -d gohypo_dev

# View tables
\dt

# View recent sessions
SELECT id, state, progress, created_at FROM research_sessions ORDER BY created_at DESC LIMIT 5;
```

## 🚀 Deployment

For production deployment:

1. Use `docker-compose.prod.yml`
2. Set environment variables securely
3. Use proper secrets management
4. Configure backups and monitoring
5. Set up proper networking and firewalls

## 📚 Additional Resources

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [pgAdmin Documentation](https://www.pgadmin.org/docs/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)