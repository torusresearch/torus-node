package auth

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/intel-go/fastjson"
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
	clientID string
	client   *http.Client
	Endpoint string
	Timeout  time.Duration
}

// GoogleVerifierParams - expected params for the google verifier
type GoogleVerifierParams struct {
	IDToken string `json:"idtoken"`
	Email   string `json:"email"`
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
func (g *GoogleVerifier) VerifyRequestIdentity(rawPayload *fastjson.RawMessage) (bool, string, error) {
	var p GoogleVerifierParams
	if err := fastjson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	p.IDToken = g.CleanToken(p.IDToken)

	if p.Email == "" || p.IDToken == "" {
		return false, "", errors.New("invalid payload parameters")
	}

	resp, err := g.client.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + p.IDToken)
	if err != nil {
		return false, "", err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	var body GoogleAuthResponse
	err = fastjson.Unmarshal(b, &body)
	if err != nil {
		return false, "", err
	}

	// Check if auth token has been signed within declared parameter
	timeSignedInt, err := strconv.Atoi(body.Iat)
	if err != nil {
		return false, "", err
	}
	timeSigned := time.Unix(int64(timeSignedInt), 0)
	if timeSigned.Add(g.Timeout).Before(time.Now()) {
		return false, "", errors.New("timesigned is more than 60 seconds ago " + timeSigned.String())
	}

	if strings.Compare(g.clientID, body.Azp) != 0 {
		return false, "", errors.New("azip is not clientID " + body.Azp + " " + g.clientID)
	}
	if strings.Compare(p.Email, body.Email) != 0 {
		return false, "", errors.New("email not equal to body.email " + p.Email + " " + body.Email)
	}

	return true, p.Email, nil
}

// NewDefaultGoogleVerifier - Constructor for the default google verifier
func NewDefaultGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{
		Version:  "1.0",
		client:   http.DefaultClient,
		clientID: clientID,
		Endpoint: "https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=",
		Timeout:  60 * time.Second,
	}
}
