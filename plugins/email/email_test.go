package email

import (
	"context"
	"testing"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/gomail.v2"
)

// mockSender implements the Sender interface for testing.
type mockSender struct {
	called       bool
	err          error
	lastMessage  *gomail.Message
	callCount    int
}

func (m *mockSender) DialAndSend(msg *gomail.Message) error {
	m.called = true
	m.callCount++
	m.lastMessage = msg
	return m.err
}

func TestPlugin(t *testing.T) {
	tests := []struct {
		name string
		opts []EmailOption
	}{
		{
			name: "default configuration loads from config",
			opts: nil,
		},
		{
			name: "with SMTP override",
			opts: []EmailOption{
				WithSMTP("smtp.example.com", 587, "user", "pass"),
			},
		},
		{
			name: "with From override",
			opts: []EmailOption{
				WithFrom("custom@example.com"),
			},
		},
		{
			name: "with custom sender",
			opts: []EmailOption{
				WithSender(&mockSender{}),
			},
		},
		{
			name: "with all options",
			opts: []EmailOption{
				WithSMTP("smtp.example.com", 587, "user", "pass"),
				WithFrom("custom@example.com"),
				WithSender(&mockSender{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(tt.opts...)
			assert.NotNil(t, p)
			assert.Equal(t, PluginName, p.Name())
		})
	}
}

func TestEmailPlugin_Name(t *testing.T) {
	p := Plugin()
	assert.Equal(t, PluginName, p.Name())
}

func TestEmailPlugin_Init(t *testing.T) {
	ctx := context.Background()
	registry := &prefab.Registry{}

	tests := []struct {
		name          string
		setupPlugin   func() *EmailPlugin
		expectedError string
	}{
		{
			name: "missing from address",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom(""),
					WithSMTP("smtp.example.com", 587, "user", "pass"),
				)
			},
			expectedError: "email: config missing from adddress",
		},
		{
			name: "missing smtp host",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("test@example.com"),
					WithSMTP("", 587, "user", "pass"),
				)
			},
			expectedError: "email: config missing smtp host",
		},
		{
			name: "missing smtp port",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("test@example.com"),
					WithSMTP("smtp.example.com", 0, "user", "pass"),
				)
			},
			expectedError: "email: config missing smtp port",
		},
		{
			name: "missing smtp username",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("test@example.com"),
					WithSMTP("smtp.example.com", 587, "", "pass"),
				)
			},
			expectedError: "email: config missing smtp username",
		},
		{
			name: "missing smtp password",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("test@example.com"),
					WithSMTP("smtp.example.com", 587, "user", ""),
				)
			},
			expectedError: "email: config missing smtp password",
		},
		{
			name: "successful initialization",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("test@example.com"),
					WithSMTP("smtp.example.com", 587, "user", "pass"),
				)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setupPlugin()
			err := p.Init(ctx, registry)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWithSMTP(t *testing.T) {
	host := "smtp.example.com"
	port := 587
	username := "testuser"
	password := "testpass"

	p := Plugin(WithSMTP(host, port, username, password))

	assert.Equal(t, host, p.smtpHost)
	assert.Equal(t, port, p.smtpPort)
	assert.Equal(t, username, p.smtpUsername)
	assert.Equal(t, password, p.smtpPassword)
}

func TestWithFrom(t *testing.T) {
	from := "custom@example.com"
	p := Plugin(WithFrom(from))
	assert.Equal(t, from, p.from)
}

func TestWithSender(t *testing.T) {
	mockSender := &mockSender{}
	p := Plugin(WithSender(mockSender))
	assert.Equal(t, mockSender, p.sender)
}

func TestEmailPlugin_Send(t *testing.T) {
	ctx := logging.EnsureLogger(context.Background())

	tests := []struct {
		name           string
		setupPlugin    func() *EmailPlugin
		setupMessage   func() *gomail.Message
		expectedError  bool
		validateSender func(*testing.T, *mockSender)
	}{
		{
			name: "successful send with custom sender",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("default@example.com"),
					WithSMTP("smtp.example.com", 587, "user", "pass"),
					WithSender(&mockSender{}),
				)
			},
			setupMessage: func() *gomail.Message {
				msg := gomail.NewMessage()
				msg.SetHeader("To", "recipient@example.com")
				msg.SetHeader("Subject", "Test Subject")
				msg.SetBody("text/plain", "Test body")
				return msg
			},
			expectedError: false,
			validateSender: func(t *testing.T, m *mockSender) {
				assert.True(t, m.called)
				assert.Equal(t, 1, m.callCount)
				assert.NotNil(t, m.lastMessage)
			},
		},
		{
			name: "sets default from address when not provided",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("default@example.com"),
					WithSMTP("smtp.example.com", 587, "user", "pass"),
					WithSender(&mockSender{}),
				)
			},
			setupMessage: func() *gomail.Message {
				msg := gomail.NewMessage()
				msg.SetHeader("To", "recipient@example.com")
				msg.SetHeader("Subject", "Test Subject")
				msg.SetBody("text/plain", "Test body")
				// Don't set From header
				return msg
			},
			expectedError: false,
			validateSender: func(t *testing.T, m *mockSender) {
				assert.True(t, m.called)
				from := m.lastMessage.GetHeader("From")
				assert.Len(t, from, 1)
				assert.Equal(t, "default@example.com", from[0])
			},
		},
		{
			name: "preserves custom from address when provided",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("default@example.com"),
					WithSMTP("smtp.example.com", 587, "user", "pass"),
					WithSender(&mockSender{}),
				)
			},
			setupMessage: func() *gomail.Message {
				msg := gomail.NewMessage()
				msg.SetHeader("From", "custom@example.com")
				msg.SetHeader("To", "recipient@example.com")
				msg.SetHeader("Subject", "Test Subject")
				msg.SetBody("text/plain", "Test body")
				return msg
			},
			expectedError: false,
			validateSender: func(t *testing.T, m *mockSender) {
				assert.True(t, m.called)
				from := m.lastMessage.GetHeader("From")
				assert.Len(t, from, 1)
				assert.Equal(t, "custom@example.com", from[0])
			},
		},
		{
			name: "handles sender error",
			setupPlugin: func() *EmailPlugin {
				return Plugin(
					WithFrom("default@example.com"),
					WithSMTP("smtp.example.com", 587, "user", "pass"),
					WithSender(&mockSender{err: assert.AnError}),
				)
			},
			setupMessage: func() *gomail.Message {
				msg := gomail.NewMessage()
				msg.SetHeader("To", "recipient@example.com")
				msg.SetHeader("Subject", "Test Subject")
				msg.SetBody("text/plain", "Test body")
				return msg
			},
			expectedError: true,
			validateSender: func(t *testing.T, m *mockSender) {
				assert.True(t, m.called)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setupPlugin()
			msg := tt.setupMessage()

			err := p.Send(ctx, msg)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.validateSender != nil {
				mockSender := p.sender.(*mockSender)
				tt.validateSender(t, mockSender)
			}
		})
	}
}

func TestEmailPlugin_Send_WithDefaultDialer(t *testing.T) {
	// This test verifies that when no custom sender is provided,
	// the plugin creates a gomailDialer at Send time.
	ctx := logging.EnsureLogger(context.Background())

	p := Plugin(
		WithFrom("test@example.com"),
		WithSMTP("smtp.example.com", 587, "user", "pass"),
		// No WithSender() - should use default gomailDialer
	)

	assert.Nil(t, p.sender, "sender should be nil before Send")

	msg := gomail.NewMessage()
	msg.SetHeader("To", "recipient@example.com")
	msg.SetHeader("Subject", "Test")
	msg.SetBody("text/plain", "Body")

	// This will fail because we don't have a real SMTP server,
	// but we're testing that the code path is correct
	err := p.Send(ctx, msg)

	// We expect an error because there's no real SMTP server
	// The important part is that it tried to connect (didn't panic)
	assert.Error(t, err, "should error when connecting to fake SMTP")
}

func TestGomailDialer(t *testing.T) {
	// Test that gomailDialer correctly wraps gomail.Dialer
	dialer := gomail.NewDialer("smtp.example.com", 587, "user", "pass")
	wrapper := &gomailDialer{dialer: dialer}

	msg := gomail.NewMessage()
	msg.SetHeader("To", "test@example.com")
	msg.SetHeader("Subject", "Test")
	msg.SetBody("text/plain", "Body")

	// This will fail because we don't have a real SMTP server
	err := wrapper.DialAndSend(msg)
	assert.Error(t, err, "should error when connecting to fake SMTP")
}

func TestSenderInterface(t *testing.T) {
	// Compile-time check that mockSender implements Sender
	var _ Sender = (*mockSender)(nil)

	// Compile-time check that gomailDialer implements Sender
	var _ Sender = (*gomailDialer)(nil)
}
