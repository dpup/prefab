// Package email provides an interface for plugins and application code to send
// email. [Gomail](gopkg.in/gomail.v2) is used with SMTP as the default.
//
// SMTP can be configured using the Viper configuration, either as ENV or from
// a configuration file.
//
// |---------------------|---------------------|
// | Env                 | JSON                |
// | --------------------|---------------------|
// | EMAIL_FROM          | email.from          |
// | EMAIL_SMTP_HOST     | email.smtp.host     |
// | EMAIL_SMTP_PORT     | email.smtp.port     |
// | EMAIL_SMTP_USERNAME | email.smtp.username |
// | EMAIL_SMTP_PASSWORD | email.smtp.password |
// |---------------------|---------------------|
package email

import (
	"context"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugin"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/gomail.v2"
)

// Constant name for identifying the email plugin.
const PluginName = "email"

// Plugin returns a new EmailPlugin.
func Plugin() *EmailPlugin {
	// TODO: Make smtp optional and allow a gomail.SendFunc to be configured.
	p := &EmailPlugin{
		from:         viper.GetString("email.from"),
		smtpHost:     viper.GetString("email.smtp.host"),
		smtpPort:     viper.GetInt("email.smtp.port"),
		smtpUsername: viper.GetString("email.smtp.username"),
		smtpPassword: viper.GetString("email.smtp.password"),
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
}

// From plugin.Plugin
func (p *EmailPlugin) Name() string {
	return PluginName
}

// From plugin.InitializablePlugin
func (p *EmailPlugin) Init(ctx context.Context, r *plugin.Registry) error {
	if p.from == "" {
		return status.Error(codes.InvalidArgument, "email: config missing from adddress")
	}
	if p.smtpHost == "" {
		return status.Error(codes.InvalidArgument, "email: config missing smtp host")
	}
	if p.smtpPort == 0 {
		return status.Error(codes.InvalidArgument, "email: config missing smtp port")
	}
	if p.smtpUsername == "" {
		return status.Error(codes.InvalidArgument, "email: config missing smtp username")
	}
	if p.smtpPassword == "" {
		return status.Error(codes.InvalidArgument, "email: config missing smtp password")
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
	d := gomail.NewDialer(p.smtpHost, p.smtpPort, p.smtpUsername, p.smtpPassword)
	if err := d.DialAndSend(msg); err != nil {
		return err
	}
	return nil
}
