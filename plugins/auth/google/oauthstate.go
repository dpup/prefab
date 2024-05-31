package google

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

const (
	stateExpiration = time.Minute * 5
)

// Wraps client state with extra information needed by the server side flow.
type oauthState struct {
	OriginalState string    `json:"s"`
	RequestUri    string    `json:"r"`
	TimeStamp     time.Time `json:"t"`
	Signature     string    `json:"sig"`
}

func (s *oauthState) Encode() string {
	b, _ := json.Marshal(s)
	return base64.StdEncoding.EncodeToString(b)
}

func (p *GooglePlugin) newOauthState(code string, redirectUri string) *oauthState {
	s := &oauthState{
		OriginalState: redirectUri,
		RequestUri:    code,
		TimeStamp:     time.Now(),
	}

	// Use the client secret to sign the state.
	h := hmac.New(sha256.New, []byte(p.clientSecret))
	h.Write([]byte(s.Encode()))
	s.Signature = hex.EncodeToString(h.Sum(nil))

	return s
}

func (p *GooglePlugin) parseState(s string) (*oauthState, error) {
	if s == "" {
		return nil, errors.NewC("google: state parameter is empty", codes.InvalidArgument)
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.NewC("google: invalid state parameter, not base64 encoded", codes.InvalidArgument)
	}
	var state oauthState
	err = json.Unmarshal(b, &state)
	if err != nil {
		return nil, errors.NewC("google: invalid state parameter, json decode failed", codes.InvalidArgument)
	}
	if state.TimeStamp.Add(stateExpiration).Before(time.Now()) {
		return nil, errors.NewC("google: state parameter has expired", codes.InvalidArgument)
	}

	actual, err := hex.DecodeString(state.Signature)
	if err != nil {
		return nil, errors.NewC("google: state parameter has invalid signature", codes.InvalidArgument)
	}
	state.Signature = ""

	h := hmac.New(sha256.New, []byte(p.clientSecret))
	h.Write([]byte(state.Encode()))
	expected := h.Sum(nil)

	if !hmac.Equal(actual, expected) {
		return nil, errors.NewC("google: state parameter has invalid signature", codes.InvalidArgument)
	}

	return &state, nil
}
