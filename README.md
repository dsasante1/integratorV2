# Integrator V2

A robust API server built with Go and Echo framework that provides integration services with built-in security features, task queue processing, and database management.

## Features

- RESTful API with Echo framework
- Built-in security features including rate limiting and email validation
- Task queue processing with worker implementation
- Database integration
- Graceful shutdown handling
- CORS support
- Gzip compression
- Request logging and recovery middleware

## Prerequisites

- Go 1.x or higher
- PostgreSQL (or your preferred database)
- Redis (for task queue)

## Environment Variables

- `PORT` - Server port (defaults to 8080)
- `DB_HOST` - Database host (defaults to localhost)
- `DB_PORT` - Database port (defaults to 5432)
- `DB_USER` - Database user (defaults to postgres)
- `DB_PASSWORD` - Database password
- `DB_NAME` - Database name (defaults to mydb)

## Database Migrations

This project uses [golang-migrate](https://github.com/golang-migrate/migrate) for database migrations. The following commands are available through mage:

```bash
# Run all pending migrations
mage migrateUp

# Roll back the last migration
mage migrateDown

# Create new migration files
mage migrateCreate <name>

# Force the database to a specific version (use carefully)
mage migrateForce <version>

# Show current migration version
mage migrateVersion

# Drop the entire database (use with extreme caution)
mage migrateDrop
```

## Project Structure

```
integratorV2/
├── internal/
│   ├── db/         # Database operations
│   ├── queue/      # Task queue implementation
│   ├── routes/     # API route definitions
│   ├── security/   # Security middleware and features
│   └── worker/     # Background task worker
└── server.go       # Main application entry point
```

## API Endpoints

The API is versioned and accessible under the `/integrator/api/v1` base path.

## Security Features

- Rate limiting to prevent abuse
- Email validation middleware
- CORS protection
- Request logging
- Panic recovery

## Getting Started

1. Clone the repository
2. Set up your environment variables
3. Initialize the database
4. Run the server:

```bash
go run server.go
```

## Graceful Shutdown

The server implements graceful shutdown with a 30-second timeout. It handles:
- SIGINT and SIGTERM signals
- Worker shutdown
- Database connection cleanup
- Queue connection cleanup

## Middleware

The server uses several middleware components:
- Logger
- Recover
- Gzip compression
- CORS
- Rate Limiter
- Email Validation

## License

[Add your license here]

## Contributing

[Add contribution guidelines here] 