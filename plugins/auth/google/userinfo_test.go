package google

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserInfoFromClaims(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]interface{}
		expectedError bool
		validateUI    func(*testing.T, *UserInfo)
	}{
		{
			name: "complete claims",
			claims: map[string]interface{}{
				"sub":            "123456",
				"email":          "test@example.com",
				"name":           "Test User",
				"given_name":     "Test",
				"family_name":    "User",
				"locale":         "en",
				"picture":        "https://example.com/pic.jpg",
				"hd":             "example.com",
				"email_verified": true,
			},
			expectedError: false,
			validateUI: func(t *testing.T, ui *UserInfo) {
				assert.Equal(t, "123456", ui.ID)
				assert.Equal(t, "test@example.com", ui.Email)
				assert.Equal(t, "Test User", ui.Name)
				assert.Equal(t, "Test", ui.GivenName)
				assert.Equal(t, "User", ui.FamilyName)
				assert.Equal(t, "en", ui.Locale)
				assert.Equal(t, "https://example.com/pic.jpg", ui.Picture)
				assert.Equal(t, "example.com", ui.Hd)
				assert.NotNil(t, ui.EmailVerified)
				assert.True(t, *ui.EmailVerified)
			},
		},
		{
			name: "minimal claims",
			claims: map[string]interface{}{
				"sub":   "123456",
				"email": "test@example.com",
				"name":  "Test User",
			},
			expectedError: false,
			validateUI: func(t *testing.T, ui *UserInfo) {
				assert.Equal(t, "123456", ui.ID)
				assert.Equal(t, "test@example.com", ui.Email)
				assert.Equal(t, "Test User", ui.Name)
				assert.Empty(t, ui.GivenName)
				assert.Empty(t, ui.FamilyName)
				assert.Nil(t, ui.EmailVerified)
			},
		},
		{
			name: "missing sub",
			claims: map[string]interface{}{
				"email": "test@example.com",
				"name":  "Test User",
			},
			expectedError: true,
		},
		{
			name: "missing email",
			claims: map[string]interface{}{
				"sub":  "123456",
				"name": "Test User",
			},
			expectedError: true,
		},
		{
			name: "missing name",
			claims: map[string]interface{}{
				"sub":   "123456",
				"email": "test@example.com",
			},
			expectedError: true,
		},
		{
			name: "email_verified false",
			claims: map[string]interface{}{
				"sub":            "123456",
				"email":          "test@example.com",
				"name":           "Test User",
				"email_verified": false,
			},
			expectedError: false,
			validateUI: func(t *testing.T, ui *UserInfo) {
				assert.NotNil(t, ui.EmailVerified)
				assert.False(t, *ui.EmailVerified)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui, err := UserInfoFromClaims(tt.claims)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ui)
				if tt.validateUI != nil {
					tt.validateUI(t, ui)
				}
			}
		})
	}
}

func TestUserInfoFromJSON(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		expectedError bool
		validateUI    func(*testing.T, *UserInfo)
	}{
		{
			name: "valid JSON",
			json: `{
				"sub": "123456",
				"email": "test@example.com",
				"name": "Test User",
				"given_name": "Test",
				"family_name": "User",
				"picture": "https://example.com/pic.jpg"
			}`,
			expectedError: false,
			validateUI: func(t *testing.T, ui *UserInfo) {
				assert.Equal(t, "123456", ui.ID)
				assert.Equal(t, "test@example.com", ui.Email)
				assert.Equal(t, "Test User", ui.Name)
			},
		},
		{
			name:          "invalid JSON",
			json:          `{"invalid json`,
			expectedError: true,
		},
		{
			name:          "empty JSON",
			json:          `{}`,
			expectedError: false,
			validateUI: func(t *testing.T, ui *UserInfo) {
				assert.Empty(t, ui.ID)
				assert.Empty(t, ui.Email)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.json)
			ui, err := UserInfoFromJSON(reader)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ui)
				if tt.validateUI != nil {
					tt.validateUI(t, ui)
				}
			}
		})
	}
}

func TestUserInfo_IsConfirmed(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		userInfo UserInfo
		expected bool
	}{
		{
			name: "Gmail account",
			userInfo: UserInfo{
				Email: "test@gmail.com",
			},
			expected: true,
		},
		{
			name: "Google Workspace with verified email",
			userInfo: UserInfo{
				Email:         "test@example.com",
				Hd:            "example.com",
				EmailVerified: &trueVal,
			},
			expected: true,
		},
		{
			name: "Google Workspace with unverified email",
			userInfo: UserInfo{
				Email:         "test@example.com",
				Hd:            "example.com",
				EmailVerified: &falseVal,
			},
			expected: false,
		},
		{
			name: "Google Workspace without email_verified",
			userInfo: UserInfo{
				Email: "test@example.com",
				Hd:    "example.com",
			},
			expected: false,
		},
		{
			name: "Third-party email with verification",
			userInfo: UserInfo{
				Email:         "test@thirdparty.com",
				EmailVerified: &trueVal,
			},
			expected: false,
		},
		{
			name: "Third-party email without verification",
			userInfo: UserInfo{
				Email: "test@thirdparty.com",
			},
			expected: false,
		},
		{
			name: "Gmail with hd set",
			userInfo: UserInfo{
				Email:         "test@gmail.com",
				Hd:            "gmail.com",
				EmailVerified: &trueVal,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.userInfo.IsConfirmed()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClaimsString(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		claims        map[string]interface{}
		required      bool
		expectedValue string
		expectedError bool
	}{
		{
			name: "required field present",
			key:  "email",
			claims: map[string]interface{}{
				"email": "test@example.com",
			},
			required:      true,
			expectedValue: "test@example.com",
			expectedError: false,
		},
		{
			name:          "required field missing",
			key:           "email",
			claims:        map[string]interface{}{},
			required:      true,
			expectedValue: "",
			expectedError: true,
		},
		{
			name:          "optional field missing",
			key:           "picture",
			claims:        map[string]interface{}{},
			required:      false,
			expectedValue: "",
			expectedError: false,
		},
		{
			name: "optional field present",
			key:  "picture",
			claims: map[string]interface{}{
				"picture": "https://example.com/pic.jpg",
			},
			required:      false,
			expectedValue: "https://example.com/pic.jpg",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := claimsString(tt.key, tt.claims, tt.required)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}
