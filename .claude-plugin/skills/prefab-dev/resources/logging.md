# Logging

Prefab provides a structured logging system built on uber-go/zap with context-aware logging capabilities.

## Server Configuration

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/logging"
)

func main() {
    // Production JSON logging
    server := prefab.New(
        prefab.WithLogger(logging.NewProdLogger()),
        prefab.WithPort(8080),
    )

    // Development console logging
    server := prefab.New(
        prefab.WithLogger(logging.NewDevLogger()),
        prefab.WithPort(8080),
    )
}
```

## Logging in Handlers

Use context-aware logging functions:

```go
import "github.com/dpup/prefab/logging"

func (s *myServiceImpl) MyRPCMethod(ctx context.Context, req *MyRequest) (*MyResponse, error) {
    // Simple info log
    logging.Info(ctx, "Processing request")

    // Structured logging with fields
    logging.Infow(ctx, "User action", "userID", req.UserId, "action", "create")

    // Formatted logging
    logging.Infof(ctx, "Processing %d items", len(items))

    // Error logging
    if err := doSomething(); err != nil {
        logging.Errorw(ctx, "Operation failed", "error", err)
        return nil, err
    }

    return &MyResponse{}, nil
}
```

## Logging Levels

- **Debug**: `logging.Debug(ctx, msg)`, `logging.Debugw(ctx, msg, fields...)`, `logging.Debugf(ctx, msg, args...)`
- **Info**: `logging.Info(ctx, msg)`, `logging.Infow(ctx, msg, fields...)`, `logging.Infof(ctx, msg, args...)`
- **Warn**: `logging.Warn(ctx, msg)`, `logging.Warnw(ctx, msg, fields...)`, `logging.Warnf(ctx, msg, args...)`
- **Error**: `logging.Error(ctx, msg)`, `logging.Errorw(ctx, msg, fields...)`, `logging.Errorf(ctx, msg, args...)`

## Structured Logging

Always prefer structured logging (`*w` variants):

```go
// Good: Structured fields
logging.Infow(ctx, "Order created",
    "orderID", order.ID,
    "userID", user.ID,
    "amount", order.Total,
)

// Avoid: Formatted strings
logging.Infof(ctx, "Order %s created for user %s", order.ID, user.ID)
```

## Field Tracking

Track fields across the lifetime of a request:

```go
func (s *myServiceImpl) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // Track the order ID across all logs in this request
    logging.Track(ctx, "orderID", newOrderID)

    // All subsequent logs include orderID
    if err := validateOrder(ctx, req); err != nil {
        logging.Error(ctx, "Validation failed") // Includes orderID
        return nil, err
    }

    return order, nil
}
```

**Warning**: Do not use `logging.Track()` in loops without creating a new scope first.

## Logging Scopes

Create named scopes for better organization:

```go
func processUsers(ctx context.Context, users []*User) {
    for _, u := range users {
        userCtx := logging.With(ctx, logging.FromContext(ctx).Named(u.ID))
        processUser(userCtx, u)
    }
}
```

## Best Practices

1. **Always use context-aware logging**: `logging.Info(ctx, ...)` not `logger.Info(...)`
2. **Prefer structured logging**: Use `Infow()` with key-value pairs
3. **Add context early**: Track important fields early in the request lifecycle
4. **Create scopes for loops**: Use `logging.With(ctx, logger.Named(...))` to avoid field pollution
5. **Log errors with context**: Include relevant fields when logging errors

For complete documentation, see [/docs/logging.md](/docs/logging.md).
