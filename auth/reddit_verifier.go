package auth

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-node/config"
)

type RedditAuthResponse struct {
	Name          string `json:"name"`
	OAuthClientID string `json:"oauth_client_id"`
}

type RedditVerifier struct{}

type RedditVerifierParams struct {
	IDToken    string `json:"idtoken"`
	VerifierID string `json:"verifier_id"`
}

func (r *RedditVerifier) GetIdentifier() string {
	return "reddit"
}

func (r *RedditVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

func (r *RedditVerifier) VerifyRequestIdentity(rawPayload *bijson.RawMessage) (bool, string, error) {
	var p RedditVerifierParams
	if err := bijson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	req, err := http.NewRequest("GET", "https://oauth.reddit.com/api/v1/me", nil)
	if err != nil {
		return false, "", err
	}

	req.Header.Set("User-Agent", "linux:torus:v0.0.1 (by /u/reddit)")
	req.Header.Set("Host", "reddit.com")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.IDToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}

	var res RedditAuthResponse
	if err := bijson.Unmarshal(body, &res); err != nil {
		return false, "", err
	}

	if !strings.EqualFold(p.VerifierID, res.Name) {
		return false, "", fmt.Errorf("Ids do not match %s %s", p.VerifierID, res.Name)
	}

	if config.GlobalMutableConfig.GetS("RedditClientID") != res.OAuthClientID {
		return false, "", fmt.Errorf("OAuth Client IDs do not match %s %s", config.GlobalMutableConfig.GetS("RedditClientID"), res.OAuthClientID)
	}

	return true, p.VerifierID, nil
}

func NewRedditVerifier() *RedditVerifier {
	return &RedditVerifier{}
}
