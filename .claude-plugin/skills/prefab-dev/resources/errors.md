# Error Handling

Prefab provides an enhanced error package with stack traces, gRPC codes, HTTP status codes, and structured log fields.

## Basic Error Creation

```go
import (
    "github.com/dpup/prefab/errors"
    "google.golang.org/grpc/codes"
)

// Simple error with stack trace
err := errors.New("something went wrong")

// Error with gRPC code
err := errors.NewC("invalid input", codes.InvalidArgument)

// Wrap existing error
err := errors.Wrap(existingErr, 0)

// Add gRPC code to existing error
err = errors.WithCode(err, codes.NotFound)

// Set user-presentable message
err = errors.WithUserPresentableMessage(err, "Resource not found")

// Override HTTP status code
err = errors.WithHTTPStatusCode(err, 404)
```

## Adding Log Fields

Attach structured log fields that the logging middleware automatically unpacks:

```go
// Add a single field
err := errors.New("database connection failed").
    WithLogField("database", "users_db")

// Add multiple fields
err := errors.New("payment processing failed").WithLogFields(map[string]interface{}{
    "user_id":    req.UserId,
    "payment_id": payment.ID,
    "amount":     payment.Amount,
})

// Chain with other error methods
err := errors.NewC("validation failed", codes.InvalidArgument).
    WithLogField("field", "email").
    WithUserPresentableMessage("Invalid email address")

// Add fields to existing errors
if err := processPayment(payment); err != nil {
    return errors.WithLogField(err, "payment_id", payment.ID)
}
```

## Common Patterns

### Database Errors

```go
func (s *Server) GetDocument(ctx context.Context, req *pb.GetDocRequest) (*pb.Document, error) {
    doc, err := s.db.GetDocument(req.DocumentId)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.NewC("document not found", codes.NotFound).
                WithLogField("document_id", req.DocumentId).
                WithUserPresentableMessage("Document not found")
        }
        return nil, errors.Wrap(err, 0).
            WithCode(codes.Internal).
            WithLogField("document_id", req.DocumentId).
            WithUserPresentableMessage("Failed to retrieve document")
    }
    return doc, nil
}
```

### External API Errors

```go
func (s *Server) ProcessPayment(ctx context.Context, req *pb.PaymentRequest) (*pb.Payment, error) {
    resp, err := s.paymentClient.Charge(ctx, req)
    if err != nil {
        return nil, errors.Wrap(err, 0).
            WithCode(codes.Internal).
            WithLogFields(map[string]interface{}{
                "provider":    "stripe",
                "amount":      req.Amount,
                "customer_id": req.CustomerId,
            }).
            WithUserPresentableMessage("Payment processing failed")
    }
    return resp, nil
}
```

### Validation Errors

```go
func validateUser(user *User) error {
    if user.Email == "" {
        return errors.NewC("email is required", codes.InvalidArgument).
            WithLogField("user_id", user.ID).
            WithUserPresentableMessage("Email address is required")
    }

    if !emailRegex.MatchString(user.Email) {
        return errors.NewC("invalid email format", codes.InvalidArgument).
            WithLogField("email_value", user.Email).
            WithUserPresentableMessage("Invalid email address format")
    }

    return nil
}
```

## Log Output

When an error with log fields is returned, the logging middleware outputs:

```json
{
  "level": "error",
  "msg": "finished call with code Internal",
  "error.type": "*errors.Error",
  "error.http_status": 500,
  "error.message": "database connection timeout",
  "error.stack_trace": ["server.go:123", "handler.go:45"],
  "user_id": "usr_123",
  "order_type": "subscription"
}
```

## Best Practices

1. **Add contextual fields** for debugging and observability
2. **Don't log sensitive data** (passwords, tokens)
3. **Use consistent field names** across your application
4. **Add fields at the point of error creation**
5. **Separate user-facing messages** from internal error details
6. **Use meaningful field names** that are searchable in logs
