package auth

import (
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-node/config"
)

// GoogleAuthResponse - expected response body from google endpoint when checking submitted token
type GoogleAuthResponse struct {
	Azp           string `json:"azp"`
	Email         string `json:"email"`
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	EmailVerified string `json:"email_verified"`
	AtHash        string `json:"at_hash"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	Locale        string `json:"locale"`
	Iat           string `json:"iat"`
	Exp           string `json:"exp"`
	Jti           string `json:"jti"`
	Alg           string `json:"alg"`
	Kid           string `json:"kid"`
	Typ           string `json:"typ"`
}

// GoogleOAuthEndpoint - endpoint for checking tokens
const GoogleOAuthEndpoint = "https://www.googleapis.com/oauth2/v3"

// GoogleVerifier - Google verifier details
type GoogleVerifier struct {
	Version  string
	Endpoint string
	Timeout  time.Duration
}

// GoogleVerifierParams - expected params for the google verifier
type GoogleVerifierParams struct {
	IDToken    string `json:"idtoken"`
	VerifierID string `json:"verifier_id"`
}

// GetIdentifier - get identifier string for verifier
func (g *GoogleVerifier) GetIdentifier() string {
	return "google"
}

// CleanToken - trim spaces to prevent replay attacks
func (g *GoogleVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

// VerifyRequestIdentity - verifies identity of user based on their token
func (g *GoogleVerifier) VerifyRequestIdentity(rawPayload *bijson.RawMessage) (bool, string, error) {
	var p GoogleVerifierParams
	if err := bijson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	p.IDToken = g.CleanToken(p.IDToken)

	if p.VerifierID == "" || p.IDToken == "" {
		return false, "", errors.New("invalid payload parameters")
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + p.IDToken)
	if err != nil {
		return false, "", err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	var body GoogleAuthResponse
	err = bijson.Unmarshal(b, &body)
	if err != nil {
		return false, "", err
	}

	// Check if auth token has been signed within declared parameter
	timeSignedInt, ok := new(big.Int).SetString(body.Iat, 10)
	if !ok {
		return false, "", errors.New("Could not get timesignedint from " + body.Iat)
	}
	timeSigned := time.Unix(timeSignedInt.Int64(), 0)
	if timeSigned.Add(g.Timeout).Before(time.Now()) {
		return false, "", errors.New("timesigned is more than 60 seconds ago " + timeSigned.String())
	}

	if strings.Compare(config.GlobalMutableConfig.GetS("GoogleClientID"), body.Azp) != 0 {
		return false, "", errors.New("azip is not clientID " + body.Azp + " " + config.GlobalMutableConfig.GetS("GoogleClientID"))
	}
	if strings.Compare(p.VerifierID, body.Email) != 0 {
		return false, "", errors.New("email not equal to body.email " + p.VerifierID + " " + body.Email)
	}

	return true, p.VerifierID, nil
}

// NewGoogleVerifier - Constructor for the default google verifier
func NewGoogleVerifier() *GoogleVerifier {
	return &GoogleVerifier{
		Version:  "1.0",
		Endpoint: "https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=",
		Timeout:  60 * time.Second,
	}
}
