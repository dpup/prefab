# PostgreSQL Storage Plugin for Prefab

This package provides a PostgreSQL implementation of the `storage.Store` interface for the Prefab framework. It enables storing data in PostgreSQL with full support for the CRUUDLE (Create, Read, Update, Upsert, Delete, List, Exists) operations.

## Features

- Store model data in PostgreSQL using JSONB for efficient storage and querying
- Support for model-specific tables or a shared default table
- Automatic table creation on initialization
- Full transaction support
- Compatible with all storage operations defined in the Prefab storage interface

## Installation

Ensure you have the PostgreSQL driver installed:

```bash
go get github.com/lib/pq
```

### Database Setup

You have two options for setting up the required database schema:

1. **Automatic setup** (development): The PostgreSQL implementation will automatically create the necessary schema, tables, and indexes when first used. This is the default behavior and requires no additional setup:

   ```go
   store := postgres.New("postgres://user:password@localhost/dbname?sslmode=disable")
   ```

2. **Manual setup using migrations** (production): For production environments, you should manage schema changes using migrations. The package includes [Goose](https://github.com/pressly/goose) compatible migration files in the [migrations](./migrations/) directory. In this case, you should disable automatic table creation:

   ```go
   store := postgres.New(
       "postgres://user:password@localhost/dbname?sslmode=disable",
       postgres.WithAutoCreateTables(false),
   )
   ```

## Usage

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/storage"
    "github.com/dpup/prefab/plugins/storage/postgres"
)

func main() {
    // Option 1: Simple connection with panic on error (for applications)
    store := postgres.New(
        "postgres://user:password@localhost/dbname?sslmode=disable",
        postgres.WithPrefix("prefab_"),
        postgres.WithSchema("myapp"),
        postgres.WithAutoCreateTables(false), // Use migrations in production
    )
    
    // Option 2: Error handling connection (for tests or more controlled environments)
    store, err := postgres.SafeNew(
        "postgres://user:password@localhost/dbname?sslmode=disable",
        postgres.WithPrefix("prefab_"),
        postgres.WithSchema("myapp"),
    )
    if err != nil {
        // Handle connection error
        log.Fatalf("Failed to connect to PostgreSQL: %v", err)
    }
    
    // Register the storage plugin
    s := prefab.New(
        prefab.WithPlugin(storage.Plugin(store)),
        // Other plugins...
    )
    
    // Start your server
    if err := s.Start(); err != nil {
        panic(err)
    }
}
```

## Configuration Options

### Connection String

The PostgreSQL connection string follows the standard format:

```
postgres://[username]:[password]@[host]:[port]/[database-name]?[parameters]
```

Example:
```
postgres://postgres:secret@localhost:5432/myapp?sslmode=disable
```

### Storage Options

- `WithPrefix(prefix string)`: Set a custom prefix for table names (default: `"prefab_"`)
- `WithSchema(schema string)`: Set a custom PostgreSQL schema for tables (default: `"public"`)
- `WithAutoCreateTables(bool)`: Control whether tables, indexes, and triggers are automatically created (default: `true`)

## Model Storage

Models are stored as JSONB documents in PostgreSQL tables. The default behavior is:

1. All models are stored in a single table named `[prefix]default` unless explicitly initialized
2. For initialized models, data is stored in model-specific tables named `[prefix][model_name]`

## Database Schema

The PostgreSQL implementation creates a schema with tables, indexes and triggers needed to efficiently store model data.

For the complete database schema, please see the [migrations documentation](./migrations/README.md) and the [Goose migration files](./migrations/) which serve as the canonical documentation for the database schema.

Key PostgreSQL features used:
- `JSONB` data type for efficient JSON storage and querying
- `GIN` indexes for optimized JSON path lookups
- Automatic schema/table creation
- Triggers for automatically updating timestamps

## Testing

The PostgreSQL implementation includes tests that verify compatibility with the storage interface, but they require a real PostgreSQL database to run.

For detailed instructions on setting up and running the tests, see [TESTING.md](./TESTING.md).