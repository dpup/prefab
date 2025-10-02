# Prefab Logging Guide

Prefab provides a structured logging system built on uber-go/zap with context-aware logging capabilities. This guide shows you how to effectively use logging in Prefab applications.

## Server Configuration

Configure logging when creating your server using the `WithLogger` option:

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
        // other options...
    )

    // Development console logging
    server := prefab.New(
        prefab.WithLogger(logging.NewDevLogger()),
        prefab.WithPort(8080),
        // other options...
    )
}
```

The logger is automatically attached to the request context for all handlers and RPC methods.

## Logging in Handlers

Use the context-aware logging functions in your gRPC handlers and HTTP handlers:

```go
import (
    "context"
    "github.com/dpup/prefab/logging"
)

func (s *myServiceImpl) MyRPCMethod(ctx context.Context, req *MyRequest) (*MyResponse, error) {
    // Simple info log
    logging.Info(ctx, "Processing request")

    // Structured logging with fields
    logging.Infow(ctx, "User action", "userID", req.UserId, "action", "create")

    // Formatted logging
    logging.Infof(ctx, "Processing %d items for user %s", len(items), userID)

    // Error logging
    if err := doSomething(); err != nil {
        logging.Errorw(ctx, "Operation failed", "error", err, "userID", req.UserId)
        return nil, err
    }

    return &MyResponse{}, nil
}
```

## Logging Levels

Prefab supports standard logging levels:

- **Debug**: `logging.Debug(ctx, msg)`, `logging.Debugw(ctx, msg, fields...)`, `logging.Debugf(ctx, msg, args...)`
- **Info**: `logging.Info(ctx, msg)`, `logging.Infow(ctx, msg, fields...)`, `logging.Infof(ctx, msg, args...)`
- **Warn**: `logging.Warn(ctx, msg)`, `logging.Warnw(ctx, msg, fields...)`, `logging.Warnf(ctx, msg, args...)`
- **Error**: `logging.Error(ctx, msg)`, `logging.Errorw(ctx, msg, fields...)`, `logging.Errorf(ctx, msg, args...)`
- **Panic**: `logging.Panic(ctx, msg)`, `logging.Panicw(ctx, msg, fields...)`, `logging.Panicf(ctx, msg, args...)`
- **Fatal**: `logging.Fatal(ctx, msg)`, `logging.Fatalw(ctx, msg, fields...)`, `logging.Fatalf(ctx, msg, args...)`

## Structured Logging

Always prefer structured logging (`*w` variants) for better log aggregation and querying:

```go
// Good: Structured fields
logging.Infow(ctx, "Order created",
    "orderID", order.ID,
    "userID", user.ID,
    "amount", order.Total,
    "items", len(order.Items),
)

// Avoid: Formatted strings make it harder to query logs
logging.Infof(ctx, "Order %s created for user %s with %d items totaling $%.2f",
    order.ID, user.ID, len(order.Items), order.Total)
```

## Logging Scopes

Create logging scopes for better organization, especially in loops:

```go
func processUsers(ctx context.Context, users []*User) {
    for _, u := range users {
        // Create a named scope for each user
        userCtx := logging.With(ctx, logging.FromContext(ctx).Named(u.ID))
        processUser(userCtx, u)
    }
}

func processUser(ctx context.Context, user *User) {
    // All logs in this scope will include the user ID
    logging.Info(ctx, "Processing user") // Logs: "user123: Processing user"
}
```

## Field Tracking

Track fields across the lifetime of a request context. Tracked fields persist through the call chain:

```go
func (s *myServiceImpl) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // Track the order ID across all logs in this request
    logging.Track(ctx, "orderID", newOrderID)

    // Call other functions - they inherit the tracked field
    if err := validateOrder(ctx, req); err != nil {
        logging.Error(ctx, "Validation failed") // Includes orderID
        return nil, err
    }

    if err := saveOrder(ctx, order); err != nil {
        logging.Error(ctx, "Save failed") // Includes orderID
        return nil, err
    }

    return order, nil
}
```

**Warning**: Do not use `logging.Track()` in loops without creating a new scope first, as tracked values persist across iterations.

## Custom Loggers

Create custom logger instances for non-request contexts:

```go
import "github.com/dpup/prefab/logging"

// In a background job or initialization
func startBackgroundJob() {
    logger := logging.NewProdLogger()
    ctx := logging.With(context.Background(), logger)

    go func() {
        for {
            logging.Info(ctx, "Background job running")
            time.Sleep(time.Minute)
        }
    }()
}
```

## Logger Interface

The `logging.Logger` interface is designed for compatibility with zap but can be implemented for other logging systems:

```go
type Logger interface {
    Debug(args ...interface{})
    Debugw(msg string, keysAndValues ...interface{})
    Debugf(msg string, args ...interface{})
    Info(args ...interface{})
    Infow(msg string, keysAndValues ...interface{})
    Infof(msg string, args ...interface{})
    // ... (warn, error, panic, fatal)

    Named(name string) Logger
    With(field string, value interface{}) Logger
}
```

## Best Practices

1. **Always use context-aware logging**: Use `logging.Info(ctx, ...)` not `logger.Info(...)`
2. **Prefer structured logging**: Use `Infow()` with key-value pairs for queryable logs
3. **Add context early**: Track important fields (userID, requestID) early in the request lifecycle
4. **Create scopes for loops**: Use `logging.With(ctx, logger.Named(...))` to avoid field pollution
5. **Log errors with context**: Include relevant fields when logging errors
6. **Use appropriate levels**: Debug for verbose details, Info for normal flow, Warn for issues, Error for failures

## Example: Complete Handler

```go
func (s *orderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // Track request-specific fields
    logging.Track(ctx, "userID", req.UserId)

    logging.Infow(ctx, "Creating order", "items", len(req.Items))

    // Validate
    if err := s.validator.Validate(req); err != nil {
        logging.Warnw(ctx, "Validation failed", "error", err)
        return nil, errors.NewC(codes.InvalidArgument, err)
    }

    // Create order
    order, err := s.db.CreateOrder(ctx, req)
    if err != nil {
        logging.Errorw(ctx, "Failed to create order", "error", err)
        return nil, err
    }

    // Track the created order
    logging.Track(ctx, "orderID", order.Id)

    logging.Infow(ctx, "Order created successfully", "total", order.Total)

    return order, nil
}
```
