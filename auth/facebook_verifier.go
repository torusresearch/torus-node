package auth

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-node/config"
)

type FacebookAuthResponse struct {
	Data struct {
		AppId               string `json:"app_id,omitempty"`
		Application         string `json:"application,omitempty"`
		DataAccessExpiresAt int    `json:"data_access_expiers_at,omitempty"`
		Error               *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
		ExpiresAt int      `json:"expires_at,omitempty"`
		IsValid   bool     `json:"is_valid"`
		Scopes    []string `json:"scopes"`
		Type      string   `json:"type,omitempty"`
		UserID    string   `json:"user_id,omitempty"`
	} `json:"data"`
}

type FacebookVerifier struct{}

type FacebookVerifierParams struct {
	IDToken    string `json:"idtoken"`
	VerifierID string `json:"verifier_id"`
}

func (f *FacebookVerifier) GetIdentifier() string {
	return "facebook"
}

func (f *FacebookVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

func (f *FacebookVerifier) VerifyRequestIdentity(rawPayload *bijson.RawMessage) (bool, string, error) {
	var p FacebookVerifierParams
	if err := bijson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	p.IDToken = f.CleanToken(p.IDToken)

	if p.IDToken == "" || p.VerifierID == "" {
		return false, "", errors.New("invalid payload parameters")
	}

	url := fmt.Sprintf(
		"https://graph.facebook.com/debug_token?input_token=%s&access_token=%s|%s",
		p.IDToken,
		config.GlobalMutableConfig.GetS("FacebookAppID"),
		config.GlobalMutableConfig.GetS("FacebookAppSecret"),
	)

	res, err := http.Get(url)
	if err != nil {
		return false, "", err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, "", err
	}

	var resp FacebookAuthResponse
	if err := bijson.Unmarshal(body, &resp); err != nil {
		return false, "", err
	}

	if resp.Data.Error != nil {
		return false, "", errors.New(resp.Data.Error.Message)
	}

	if !resp.Data.IsValid {
		return false, "", errors.New("Access token is not valid")
	}

	if resp.Data.UserID != p.VerifierID {
		return false, "", errors.New("UserIDs do not match")
	}

	return true, p.VerifierID, nil
}

func NewFacebookVerifier() *FacebookVerifier {
	return &FacebookVerifier{}
}
