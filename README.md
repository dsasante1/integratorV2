# Integrator V2

A postman VCS API server built with Go and Echo framework.

## Features

- RESTful API with Echo framework
- Built-in security features including rate limiting and email validation
- Task queue processing with worker implementation
- Database integration with migration support
- Firebase/Firestore integration
- Notification service
- Graceful shutdown handling
- CORS support
- Gzip compression
- Request logging and recovery middleware

## Prerequisites

- Go 1.x or higher
- PostgreSQL (or your preferred database)
- Redis (for task queue)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI tool

### Installing golang-migrate

```bash
# Install golang-migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## Environment Variables

Create a `.env` file in your project root:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=yourdatabase
PORT=8080
```

Available environment variables:
- `PORT` - Server port (defaults to 8080)
- `DB_HOST` - Database host (defaults to localhost)
- `DB_PORT` - Database port (defaults to 5432)
- `DB_USER` - Database user (defaults to postgres)
- `DB_PASSWORD` - Database password
- `DB_NAME` - Database name (defaults to mydb)

## Database Migrations

This project includes a built-in migration system using [golang-migrate](https://github.com/golang-migrate/migrate). All migration commands are available through CLI flags:

### Migration Commands

```bash
# Run all pending migrations
go run main.go -migrate

# Roll back one migration
go run main.go -migrate-down

# Create new migration files
go run main.go -migrate-create="add_users_table"

# Show current migration version
go run main.go -migrate-version

# Force database to a specific version (use carefully)
go run main.go -migrate-force="3"

# Reset all migrations (drops all tables and re-runs migrations)
go run main.go -migrate-reset

# Drop entire database (DANGEROUS - asks for confirmation)
go run main.go -migrate-drop

# Start server with automatic migration on startup
go run main.go -auto-migrate
```

### Migration File Structure

Migration files are stored in the `migrations/` directory:

```
migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_users_table.up.sql
└── 000002_add_users_table.down.sql
```

### Example Migration Files

**000001_initial_schema.up.sql:**
```sql
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**000001_initial_schema.down.sql:**
```sql
DROP TABLE IF EXISTS users;
```

## Project Structure

```
integratorV2/
├── main.go                    # Application entry point and CLI
├── .env                       # Environment variables
├── migrations/                # Database migration files
│   ├── 000001_initial.up.sql
│   └── 000001_initial.down.sql
└── internal/
    ├── app/                   # Application logic and lifecycle
    │   ├── app.go
    │   └── routes.go
    ├── migrations/            # Migration package
    │   └── migrations.go
    ├── config/                # Configuration and Firebase
    ├── db/                    # Database operations
    ├── notification/          # Notification service
    ├── queue/                 # Task queue implementation
    ├── routes/                # API route definitions
    ├── security/              # Security middleware and features
    └── worker/                # Background task worker
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

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd integratorV2
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Set up your environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your database credentials
   ```

4. **Install golang-migrate CLI** (if not already installed)
   ```bash
   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
   ```

5. **Run database migrations**
   ```bash
   go run main.go -migrate
   ```

6. **Start the server**
   ```bash
   go run main.go
   ```

   Or start with auto-migration:
   ```bash
   go run main.go -auto-migrate
   ```

## Running the Application

### Development Mode
```bash
# Start server with auto-migration
go run main.go -auto-migrate

# Start server normally (assumes migrations are up to date)
go run main.go
```

### Production Mode
```bash
# Build the application
go build -o integrator main.go

# Run migrations
./integrator -migrate

# Start the server
./integrator
```

## Graceful Shutdown

The server implements graceful shutdown with a 30-second timeout. It handles:
- SIGINT and SIGTERM signals
- Worker shutdown
- Database connection cleanup
- Queue connection cleanup
- Firebase connection cleanup

## Middleware

The server uses several middleware components:
- Logger
- Recover
- Gzip compression
- CORS
- Rate Limiter
- Email Validation

## Services

### Database
- PostgreSQL integration with connection pooling
- Automatic migration support
- Graceful connection handling

### Task Queue
- Redis-based task queue
- Background worker processing
- Graceful worker shutdown

### Firebase/Firestore
- Firebase integration for additional data storage
- Automatic connection management

### Notifications
- Built-in notification service
- Extensible notification providers

## Development

### Adding New Migrations

```bash
# Create a new migration
go run main.go -migrate-create="add_products_table"

# Edit the generated files in migrations/ directory
# Run the migration
go run main.go -migrate
```

### Database Operations

```bash
# Check current migration version
go run main.go -migrate-version

# Roll back if needed
go run main.go -migrate-down

# Reset database (development only)
go run main.go -migrate-reset
```

## Troubleshooting

### Migration Issues

1. **Check migration version:**
   ```bash
   go run main.go -migrate-version
   ```

2. **Force to a specific version (if migrations are out of sync):**
   ```bash
   go run main.go -migrate-force="1"
   ```

3. **Reset database (development only):**
   ```bash
   go run main.go -migrate-reset
   ```

### Database Connection Issues

- Ensure PostgreSQL is running
- Check your `.env` file for correct database credentials
- Verify database exists and user has proper permissions

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]