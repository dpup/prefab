package google

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

const userInfoEndpoint = "https://www.googleapis.com/oauth2/v3/userinfo"

// UserInfoFromClaims returns a UserInfo struct from the claims map. If the
// claims are invalid or missing, an error is returned.
func UserInfoFromClaims(c map[string]interface{}) (*UserInfo, error) {
	ui := &UserInfo{}
	var err error
	ui.ID, err = claimsString("sub", c, true)
	if err != nil {
		return nil, err
	}
	ui.Email, err = claimsString("email", c, true)
	if err != nil {
		return nil, err
	}
	ui.Name, err = claimsString("name", c, true)
	if err != nil {
		return nil, err
	}
	ui.GivenName, _ = claimsString("given_name", c, false)
	ui.FamilyName, _ = claimsString("family_name", c, false)
	ui.Locale, _ = claimsString("locale", c, false)
	ui.Picture, _ = claimsString("picture", c, false)
	ui.Hd, _ = claimsString("hd", c, false)

	if verified, ok := c["email_verified"].(bool); ok {
		ui.EmailVerified = &verified
	}

	return ui, nil
}

func claimsString(key string, c map[string]interface{}, required bool) (string, error) {
	if v, ok := c[key].(string); ok {
		return v, nil
	}
	if !required {
		return "", nil
	}
	return "", errors.Codef(codes.Internal, "google: failed to decode claims: missing '%s'", key)
}

// UserInfoFromJSON returns a UserInfo struct from the JSON data. If the JSON is
// invalid, an error is returned.
func UserInfoFromJSON(data io.Reader) (*UserInfo, error) {
	userInfo := &UserInfo{}
	if err := json.NewDecoder(data).Decode(userInfo); err != nil {
		return nil, errors.Codef(codes.Internal, "google: failed to decode user info: %s", err)
	}
	return userInfo, nil
}

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

	// The user's preferred locale.
	Locale string `json:"locale,omitempty"`

	// URL of the user's picture image.
	Picture string `json:"picture,omitempty"`

	// The hosted domain e.g. example.com if the user is Google apps user.
	Hd string `json:"hd,omitempty"`

	// Included for 3rd party emails and some HD accounts.
	EmailVerified *bool `json:"email_verified,omitempty"`
}

// IsConfirmed returns true if Google is authorative and the user is confirmed
// to be a legitimate user.
//
// From https://developers.google.com/identity/gsi/web/reference/js-reference#google.accounts.id.renderButton"
//
// > Using the email, email_verified and hd fields you can determine if Google
// > hosts and is authoritative for an email address. In cases where Google is
// > authoritative the user is confirmed to be the legitimate account owner.
// >
// > Cases where Google is authoritative:
// >
// > - email has a @gmail.com suffix, this is a Gmail Account.
// > - email_verified is true and hd is set, this is a Google Workspace account.
// >
// > Users may register for Google Accounts without using Gmail or Google
// > Workspace. When email does not contain a @gmail.com suffix and hd is absent
// > Google is not authoritative and password or other challenge methods are
// > recommended to verify the user. email_verfied can also be true as Google
// > initially verified the user when the Google Account was created, however
// > ownership of the third party email account may have since changed.
//
// IsConfirmed() is conservative and only returns true if Google is authorative
// and the address has been verified. In otherwords, a user may have
// EmailVerified=true and not be confirmed. Use-case should determine whether to
// trust the verification.
func (ui *UserInfo) IsConfirmed() bool {
	// If hd is set, then Google is authoritative for the email address.
	if ui.Hd != "" {
		return ui.EmailVerified != nil && *ui.EmailVerified
	}

	// If the email has a gmail suffix then Google is authoritative.
	if strings.HasSuffix(ui.Email, "@gmail.com") {
		return true
	}

	// Google is not authoritative for the email address.
	return false
}
