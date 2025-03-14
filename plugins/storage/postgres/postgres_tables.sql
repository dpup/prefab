-- +goose Up
-- +goose StatementBegin
-- This is an example migration file for the Prefab PostgreSQL storage implementation.
-- It creates the necessary tables and indexes needed for the storage plugin to function.
-- There is a default table and an example of a dedicated table.
-- First, create a dedicated schema if it doesn't exist
CREATE SCHEMA IF NOT EXISTS prefab;

-- Create the default table that stores all models that haven't been explicitly initialized
-- This table uses a composite primary key of (id, entity_type) to allow different entity
-- types to use the same ID without conflicts
CREATE TABLE IF NOT EXISTS prefab.default (
    -- Primary key provided by the model.PK() method
    id TEXT NOT NULL,
    -- Entity type (derived from model struct name, pluralized and snake_cased)
    entity_type TEXT NOT NULL,
    -- Serialized model data as JSONB for efficient storage and querying
    value JSONB NOT NULL,
    -- Timestamps for record creation and modification
    created_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT NOW (),
        updated_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT NOW (),
        -- Primary key constraint
        PRIMARY KEY (id, entity_type)
);

-- Create an index on entity_type to speed up queries that filter by type
CREATE INDEX IF NOT EXISTS idx_prefab_default_entity_type ON prefab.default (entity_type);

-- Create a GIN index on the JSONB value to allow for efficient JSON path queries
CREATE INDEX IF NOT EXISTS idx_prefab_default_value ON prefab.default USING GIN (value jsonb_path_ops);

-- Example of a model-specific table that would be created for initialized models
-- The table name would be derived from the model name (pluralized and snake_cased)
-- This is just an example - actual tables are created dynamically as models are initialized
CREATE TABLE IF NOT EXISTS prefab.users (
    -- Primary key provided by the model.PK() method
    id TEXT NOT NULL PRIMARY KEY,
    -- Serialized model data as JSONB for efficient storage and querying
    value JSONB NOT NULL,
    -- Timestamps for record creation and modification
    created_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT NOW (),
        updated_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT NOW ()
);

-- Create a GIN index on the JSONB value to allow for efficient JSON path queries
CREATE INDEX IF NOT EXISTS idx_prefab_users_value ON prefab.users USING GIN (value jsonb_path_ops);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Drop tables
DROP TABLE IF EXISTS prefab.users;

DROP TABLE IF EXISTS prefab.default;

-- Drop schema (only if empty)
DROP SCHEMA IF EXISTS prefab CASCADE;

-- +goose StatementEnd