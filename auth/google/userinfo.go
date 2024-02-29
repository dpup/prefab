package google

const userInfoEndpoint = "https://www.googleapis.com/oauth2/v3/userinfo"

// The result of calling Google's OAuth2 userinfo endpoint response. Per google
// the email is always verified as they only return the primary email for the
// account.
type UserInfo struct {
	// The user's unique and stable ID.
	ID string `json:"sub"`

	// The user's email address.
	Email string `json:"email,omitempty"`

	// The user's full name.
	Name string `json:"name,omitempty"`

	// The user's first name.
	GivenName string `json:"given_name,omitempty"`

	// The user's last name.
	FamilyName string `json:"family_name,omitempty"`

	// The hosted domain e.g. example.com if the user is Google apps user.
	Hd string `json:"hd,omitempty"`

	// The user's preferred locale.
	Locale string `json:"locale,omitempty"`

	// URL of the user's picture image.
	Picture string `json:"picture,omitempty"`
}
