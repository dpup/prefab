# Storage

Prefab provides storage plugins for data persistence.

## In-Memory Storage

For testing and development:

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/storage"
    "github.com/dpup/prefab/plugins/storage/memstore"
)

s := prefab.New(
    prefab.WithPlugin(storage.Plugin(memstore.New())),
)
```

## SQLite Storage

For development or lightweight production:

```go
import (
    "github.com/dpup/prefab/plugins/storage/sqlite"
)

s := prefab.New(
    prefab.WithPlugin(storage.Plugin(sqlite.New("database.db"))),
)
```

## Using Storage

Access storage in your services:

```go
func (s *Server) CreateItem(ctx context.Context, req *pb.CreateItemRequest) (*pb.Item, error) {
    store, ok := storage.FromContext(ctx)
    if !ok {
        return nil, errors.NewC("storage unavailable", codes.Internal)
    }

    item := &Item{
        ID:   uuid.New().String(),
        Name: req.Name,
    }

    if err := store.Put(ctx, "items", item.ID, item); err != nil {
        return nil, errors.Wrap(err, codes.Internal)
    }

    return item.ToProto(), nil
}

func (s *Server) GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.Item, error) {
    store, ok := storage.FromContext(ctx)
    if !ok {
        return nil, errors.NewC("storage unavailable", codes.Internal)
    }

    var item Item
    if err := store.Get(ctx, "items", req.Id, &item); err != nil {
        if errors.Is(err, storage.ErrNotFound) {
            return nil, errors.NewC("item not found", codes.NotFound)
        }
        return nil, errors.Wrap(err, codes.Internal)
    }

    return item.ToProto(), nil
}
```

## Storage Interface

The storage plugin implements a simple key-value interface:

```go
type Store interface {
    Get(ctx context.Context, bucket, key string, value interface{}) error
    Put(ctx context.Context, bucket, key string, value interface{}) error
    Delete(ctx context.Context, bucket, key string) error
    List(ctx context.Context, bucket string) ([]string, error)
}
```

## Token Storage

The auth plugin uses storage for token revocation:

```go
s := prefab.New(
    prefab.WithPlugin(storage.Plugin(sqlite.New("app.db"))),
    prefab.WithPlugin(auth.Plugin()),  // Uses storage for token revocation
)
```
