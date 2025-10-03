// Package email provides an interface for plugins and application code to send
// email. [Gomail](gopkg.in/gomail.v2) is used with SMTP as the default.
//
// SMTP can be configured using global configuration, either as ENV or from
// a configuration file.
//
// |-------------------------|---------------------|
// | Env                     | JSON                |
// | ------------------------|---------------------|
// | PF__EMAIL__FROM         | email.from          |
// | PF__EMAIL__SMTP__HOST   | email.smtp.host     |
// | PF__EMAIL__SMTP__PORT   | email.smtp.port     |
// | PF__EMAIL__SMTP__USER   | email.smtp.username |
// | PF__EMAIL__SMTP__PASS   | email.smtp.password |
// |-------------------------|---------------------|
package email

import (
	"context"
	"errors"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
	"gopkg.in/gomail.v2"
)

// Constant name for identifying the email plugin.
const PluginName = "email"

// Sender is an interface for sending emails. This abstraction allows for
// testing without requiring a real SMTP connection.
type Sender interface {
	DialAndSend(*gomail.Message) error
}

// gomailDialer wraps gomail.Dialer to implement the Sender interface.
type gomailDialer struct {
	dialer *gomail.Dialer
}

func (g *gomailDialer) DialAndSend(msg *gomail.Message) error {
	return g.dialer.DialAndSend(msg)
}

// EmailOptions customize the configuration of the email plugin.
type EmailOption func(*EmailPlugin)

// WithSMTP configures the SMTP server to use.
func WithSMTP(host string, port int, username, password string) EmailOption {
	return func(p *EmailPlugin) {
		p.smtpHost = host
		p.smtpPort = port
		p.smtpUsername = username
		p.smtpPassword = password
	}
}

// WithFrom configures the default from address.
func WithFrom(from string) EmailOption {
	return func(p *EmailPlugin) {
		p.from = from
	}
}

// WithSender configures a custom Sender implementation. This is primarily
// useful for testing, allowing you to inject a mock sender.
func WithSender(sender Sender) EmailOption {
	return func(p *EmailPlugin) {
		p.sender = sender
	}
}

// Plugin returns a new EmailPlugin.
func Plugin(opts ...EmailOption) *EmailPlugin {
	cfg := prefab.Config
	p := &EmailPlugin{
		from:         cfg.String("email.from"),
		smtpHost:     cfg.String("email.smtp.host"),
		smtpPort:     cfg.Int("email.smtp.port"),
		smtpUsername: cfg.String("email.smtp.username"),
		smtpPassword: cfg.String("email.smtp.password"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// EmailPlugin exposes the ability to send emails.
type EmailPlugin struct {
	from         string
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	sender       Sender
}

// From prefab.Plugin.
func (p *EmailPlugin) Name() string {
	return PluginName
}

// From prefab.InitializablePlugin.
func (p *EmailPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	if p.from == "" {
		return errors.New("email: config missing from adddress")
	}
	if p.smtpHost == "" {
		return errors.New("email: config missing smtp host")
	}
	if p.smtpPort == 0 {
		return errors.New("email: config missing smtp port")
	}
	if p.smtpUsername == "" {
		return errors.New("email: config missing smtp username")
	}
	if p.smtpPassword == "" {
		return errors.New("email: config missing smtp password")
	}
	return nil
}

// Send an email.
// TODO: Switch to daemon mode per example here:
// https://pkg.go.dev/gopkg.in/gomail.v2#example-package-Daemon
func (p *EmailPlugin) Send(ctx context.Context, msg *gomail.Message) error {
	logging.Info(ctx, "Sending mail")
	if len(msg.GetHeader("From")) == 0 {
		msg.SetHeader("From", p.from)
	}

	// Use injected sender if available, otherwise create default gomail dialer
	sender := p.sender
	if sender == nil {
		sender = &gomailDialer{
			dialer: gomail.NewDialer(p.smtpHost, p.smtpPort, p.smtpUsername, p.smtpPassword),
		}
	}

	if err := sender.DialAndSend(msg); err != nil {
		return err
	}
	return nil
}
