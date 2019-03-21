package dkgnode

import (
	"errors"
	"net/http"

	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
)

type AuthBodyGoogle struct {
	Azp            string `json:"azp"`
	Email          string `json:"email"`
	Iss            string `json:"iss"`
	Aud            string `json:"aud"`
	Sub            string `json:"sub"`
	Email_verified string `json:"email_verified"`
	At_hash        string `json:"at_hash"`
	Name           string `json:"name"`
	Picture        string `json:"picture"`
	Given_name     string `json:"given_name"`
	Locale         string `json:"locale"`
	Iat            string `json:"iat"`
	Exp            string `json:"exp"`
	Jti            string `json:"jti"`
	Alg            string `json:"alg"`
	Kid            string `json:"kid"`
	Typ            string `json:"typ"`
}

const GoogleOAuthEndpoint = "https://www.googleapis.com/oauth2/v3"

type GoogleVerifier struct {
	clientID string
	client   *http.Client
}

func (g *GoogleVerifier) VerifyRequestIdentity(rawPayload *fastjson.RawMessage) (bool, error) {
	var p ShareRequestParams
	if err := jsonrpc.Unmarshal(rawPayload, &p); err != nil {
		return false, err
	}

	if p.Email == "" || p.IDToken == "" {
		return false, errors.New("invalid payload parameters")
	}

	return true, nil
}

func NewDefaultGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{
		client:   http.DefaultClient,
		clientID: clientID,
	}
}
