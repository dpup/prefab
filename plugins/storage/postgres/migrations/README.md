# Prefab PostgreSQL Migrations

This directory contains [Goose](https://github.com/pressly/goose) compatible migration files for the Prefab PostgreSQL storage implementation. These migrations serve as the canonical documentation for the database schema required by the PostgreSQL storage plugin.

## Usage

### Using with Goose

1. Install Goose:
   ```
   go install github.com/pressly/goose/v3/cmd/goose@latest
   ```

2. Run migrations:
   ```
   goose -dir /path/to/migrations postgres "postgres://user:password@localhost:5432/dbname?sslmode=disable" up
   ```

### Alternative Deployment

If you're not using Goose, the migration files can still serve as reference for manually setting up the required schema. The migration files are plain SQL and can be executed directly against your PostgreSQL database.

## Schema Description

The PostgreSQL storage implementation requires:

1. **A dedicated schema** (configurable, defaults to "prefab")
2. **A default table** for storing models that haven't been explicitly initialized
3. **Model-specific tables** that are created dynamically as models are initialized

### Default Table Structure

The default table (`prefab.default`) has the following structure:

| Column       | Type                       | Description                                         |
|--------------|----------------------------|-----------------------------------------------------|
| id           | TEXT                       | Primary key from model's PK() method                |
| entity_type  | TEXT                       | Entity type name (pluralized model name)            |
| value        | JSONB                      | Serialized model data                               |
| created_at   | TIMESTAMP WITH TIME ZONE   | Record creation timestamp                           |
| updated_at   | TIMESTAMP WITH TIME ZONE   | Record update timestamp                             |

This table uses a composite primary key (id, entity_type) to allow different model types to use the same ID.

### Model-Specific Table Structure

Model-specific tables (e.g., `prefab.users`) have the following structure:

| Column       | Type                       | Description                                         |
|--------------|----------------------------|-----------------------------------------------------|
| id           | TEXT                       | Primary key from model's PK() method                |
| value        | JSONB                      | Serialized model data                               |
| created_at   | TIMESTAMP WITH TIME ZONE   | Record creation timestamp                           |
| updated_at   | TIMESTAMP WITH TIME ZONE   | Record update timestamp                             |

These tables use a single-column primary key (id) since they only store one model type.

### Indexes

The following indexes are created for performance:

- `idx_prefab_default_entity_type`: Index on entity_type to speed up queries that filter by type
- `idx_prefab_default_value`: GIN index on the JSONB value for efficient JSON path queries
- `idx_prefab_*_value`: GIN indexes on each model-specific table's value column

### Triggers

Automatic triggers update the `updated_at` timestamp whenever a record is updated.

## Customization

When using the PostgreSQL storage implementation in your application, you can customize:

1. The schema name via `postgres.WithSchema("custom_schema")`
2. The table prefix via `postgres.WithPrefix("custom_prefix_")`
3. Whether tables are auto-created via `postgres.WithAutoCreateTables(bool)`

The migration file uses `prefab` as both the schema name and prefix, but these can be adjusted in your application configuration.

## Using Migrations vs. Auto-Creation

The PostgreSQL storage implementation offers two modes of operation:

1. **Auto-creation mode** (default): Tables, indexes, and triggers are automatically created when the store is initialized or when models are registered. This is convenient for development but not recommended for production.

2. **Migration mode**: Database schema is managed through migrations. In this mode, you should:
   - Run the migrations using Goose or another migration tool
   - Set `WithAutoCreateTables(false)` when initializing the store

Example:

```go
// Production setup using migrations
store := postgres.New(
    "postgres://user:password@localhost/dbname?sslmode=disable",
    postgres.WithSchema("prefab"),
    postgres.WithPrefix("myapp_"),
    postgres.WithAutoCreateTables(false), // Disable auto-creation
)
```