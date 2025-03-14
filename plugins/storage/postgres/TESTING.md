# Testing the PostgreSQL Storage Implementation

The PostgreSQL storage implementation comes with tests that verify its compatibility with the `storage.Store` interface. These tests are skipped by default because they require a real PostgreSQL database.

## Setting Up PostgreSQL for Testing

### Using Docker (Recommended)

The easiest way to set up a PostgreSQL database for testing is to use Docker:

```bash
# Start a PostgreSQL container
docker run --name prefab-postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_USER=postgres -e POSTGRES_DB=prefab -p 5432:5432 -d postgres:14

# Verify it's running
docker ps
```

### Using a Local PostgreSQL Installation

If you prefer to use a local PostgreSQL installation:

1. Install PostgreSQL using your system package manager:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install postgresql
   
   # macOS with Homebrew
   brew install postgresql
   ```

2. Create a database for testing:
   ```bash
   # Create database
   createdb prefab
   
   # Or using psql
   psql -c "CREATE DATABASE prefab;"
   ```

## Running the Tests

Once you have a PostgreSQL database set up, you can run the tests with the `PG_TEST_DSN` environment variable:

```bash
# For the Docker setup
PG_TEST_DSN="postgres://postgres:postgres@localhost:5432/prefab?sslmode=disable" go test -v ./plugins/storage/postgres

# For a custom installation with password authentication
PG_TEST_DSN="postgres://username:password@localhost:5432/prefab?sslmode=disable" go test -v ./plugins/storage/postgres

# For systems using peer authentication (common on Linux)
PG_TEST_DSN="postgres://localhost/prefab?sslmode=disable" go test -v ./plugins/storage/postgres
```

The tests will automatically:
1. Check if the database connection is available
2. Skip the tests if the connection fails (with an informative message)
3. Drop and recreate the test schema for each test case
4. Create a fresh connection for each test
5. Handle proper error translation between PostgreSQL and the storage interface

This ensures that each test runs in isolation with a clean database state, preventing test interference.

The tests will also pass on any system, even if PostgreSQL isn't available. They will only run the actual tests if a working PostgreSQL database is available.

## Troubleshooting

### Connection Issues

If you see errors related to connecting to the PostgreSQL database:

1. Verify the database is running:
   ```bash
   # For Docker
   docker ps | grep postgres
   
   # For local installation
   pg_isready
   ```

2. Test the connection with psql:
   ```bash
   psql "postgres://postgres:postgres@localhost:5432/prefab?sslmode=disable"
   ```

3. Check your firewall settings to ensure port 5432 is accessible.

### Permission Issues

If you see permission errors:

1. Verify the user has appropriate permissions:
   ```bash
   psql -c "GRANT ALL PRIVILEGES ON DATABASE prefab TO postgres;"
   ```

2. For schema creation issues, verify the user can create schemas:
   ```bash
   psql -c "ALTER USER postgres CREATEDB;"
   ```

## Cleaning Up

When you're done testing, you can clean up the Docker container:

```bash
docker stop prefab-postgres
docker rm prefab-postgres
```

For a local installation, you can drop the test database:

```bash
dropdb prefab
```