# Email

The email plugin provides SMTP-based email sending capabilities.

## Setup

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/email"
)

s := prefab.New(
    prefab.WithPlugin(email.Plugin()),
)
```

## Configuration

Via YAML:
```yaml
email:
  from: noreply@example.com
  smtp:
    host: smtp.example.com
    port: 587
    username: smtp-user
    password: smtp-password
```

Via environment variables:
```bash
export PF__EMAIL__FROM=noreply@example.com
export PF__EMAIL__SMTP__HOST=smtp.example.com
export PF__EMAIL__SMTP__PORT=587
export PF__EMAIL__SMTP__USER=smtp-user
export PF__EMAIL__SMTP__PASS=smtp-password
```

Via functional options:
```go
s := prefab.New(
    prefab.WithPlugin(email.Plugin(
        email.WithFrom("noreply@example.com"),
        email.WithSMTP("smtp.example.com", 587, "user", "pass"),
    )),
)
```

## Sending Emails

```go
import "gopkg.in/gomail.v2"

func (s *Server) SendWelcome(ctx context.Context, user *User) error {
    // Get email plugin from registry
    emailPlugin := s.registry.Get("email").(*email.EmailPlugin)

    msg := gomail.NewMessage()
    msg.SetHeader("To", user.Email)
    msg.SetHeader("Subject", "Welcome!")
    msg.SetBody("text/html", "<h1>Welcome to our service!</h1>")

    return emailPlugin.Send(ctx, msg)
}
```

## With Templates

Combine with templates plugin for dynamic emails:

```go
func (s *Server) SendOrderConfirmation(ctx context.Context, order *Order) error {
    // Render template
    body, err := s.templates.Render(ctx, "order-confirmation.tmpl", order)
    if err != nil {
        return err
    }

    msg := gomail.NewMessage()
    msg.SetHeader("To", order.CustomerEmail)
    msg.SetHeader("Subject", "Order Confirmation #" + order.ID)
    msg.SetBody("text/html", body)

    return s.email.Send(ctx, msg)
}
```

## Testing

Use a custom sender for testing:

```go
type mockSender struct {
    messages []*gomail.Message
}

func (m *mockSender) DialAndSend(msg *gomail.Message) error {
    m.messages = append(m.messages, msg)
    return nil
}

// In tests
mock := &mockSender{}
s := prefab.New(
    prefab.WithPlugin(email.Plugin(
        email.WithSender(mock),
        email.WithFrom("test@example.com"),
    )),
)
```

## Attachments

```go
msg := gomail.NewMessage()
msg.SetHeader("To", recipient)
msg.SetHeader("Subject", "Report")
msg.SetBody("text/plain", "Please find the report attached.")
msg.Attach("/path/to/report.pdf")

emailPlugin.Send(ctx, msg)
```
