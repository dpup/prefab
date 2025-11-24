# Templates

The templates plugin provides Go HTML template rendering with access to configuration.

## Setup

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/templates"
)

s := prefab.New(
    prefab.WithPlugin(templates.Plugin()),
)
```

## Configuration

Via YAML:
```yaml
templates:
  alwaysParse: false  # Set true for development to reload templates
  dirs:
    - ./templates
    - ./emails
```

Via environment variables:
```bash
export PF__TEMPLATES__ALWAYS_PARSE=true
export PF__TEMPLATES__DIRS=./templates,./emails
```

## Template Files

Create `.tmpl` files in configured directories:

```html
<!-- templates/welcome.tmpl -->
<!DOCTYPE html>
<html>
<head>
    <title>Welcome</title>
</head>
<body>
    <h1>Welcome, {{.Data.Name}}!</h1>
    <p>Your account has been created.</p>
    <p>App version: {{.Config.app.version}}</p>
</body>
</html>
```

## Rendering Templates

```go
func (s *Server) RenderWelcome(ctx context.Context, user *User) (string, error) {
    templatesPlugin := s.registry.Get("templates").(*templates.TemplatePlugin)

    return templatesPlugin.Render(ctx, "welcome.tmpl", map[string]interface{}{
        "Name":  user.Name,
        "Email": user.Email,
    })
}
```

## Template Data Access

Templates receive a `TemplateData` struct:

```go
type TemplateData struct {
    Data   interface{}            // Your data
    Config map[string]interface{} // All config values
}
```

Access data in templates:
- `.Data.FieldName` - Your passed data
- `.Config.app.setting` - Configuration values

## Common Patterns

### Email Templates

```go
s := prefab.New(
    prefab.WithPlugin(email.Plugin()),
    prefab.WithPlugin(templates.Plugin()),
)

func (s *Server) SendEmail(ctx context.Context, to, templateName string, data interface{}) error {
    body, err := s.templates.Render(ctx, templateName, data)
    if err != nil {
        return err
    }

    msg := gomail.NewMessage()
    msg.SetHeader("To", to)
    msg.SetBody("text/html", body)

    return s.email.Send(ctx, msg)
}
```

### Magic Link Authentication

The magic link auth plugin uses templates for email:

```go
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(email.Plugin()),
    prefab.WithPlugin(templates.Plugin()),
    prefab.WithPlugin(magiclink.Plugin()),
)
```

### Loading Additional Directories

```go
func (s *Server) Init(ctx context.Context, r *prefab.Registry) error {
    tp := r.Get("templates").(*templates.TemplatePlugin)
    return tp.Load([]string{"./custom-templates"})
}
```

## Development Mode

Set `alwaysParse: true` to reload templates on every render (useful for development):

```yaml
templates:
  alwaysParse: true
```
