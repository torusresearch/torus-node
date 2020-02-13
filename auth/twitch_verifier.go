package auth

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-node/config"
)

type TwitchAuthResponse struct {
	AUD string `json:"aud"`
	EXP int    `json:"exp"`
	IAT int    `json:"iat"`
	ISS string `json:"iss"`
	SUB string `json:"sub"`
	AZP string `json:"azp"`
}

type TwitchVerifier struct {
	Timeout time.Duration
}

type TwitchVerifierParams struct {
	IDToken    string `json:"idtoken"`
	VerifierID string `json:"verifier_id"`
}

func (t *TwitchVerifier) GetIdentifier() string {
	return "twitch"
}

func (t *TwitchVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

func (t *TwitchVerifier) VerifyRequestIdentity(rawPayload *bijson.RawMessage) (bool, string, error) {
	var p TwitchVerifierParams
	if err := bijson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	p.IDToken = t.CleanToken(p.IDToken)

	if p.IDToken == "" || p.VerifierID == "" {
		return false, "", errors.New("invalid payload parameters")
	}

	req, err := http.NewRequest("GET", "https://id.twitch.tv/oauth2/userinfo", nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.IDToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}

	var res TwitchAuthResponse
	if err := bijson.Unmarshal(body, &res); err != nil {
		return false, "", err
	}

	// Check if auth token has been signed within declared parameter
	timeSigned := time.Unix(int64(res.IAT), 0)
	if timeSigned.Add(t.Timeout).Before(time.Now()) {
		return false, "", errors.New("timesigned is more than 60 seconds ago " + timeSigned.String())
	}
	if res.SUB != p.VerifierID {
		return false, "", fmt.Errorf("UserIDs do not match %s %s", res.SUB, p.VerifierID)
	}

	if res.AUD != config.GlobalMutableConfig.GetS("TwitchClientID") {
		return false, "", fmt.Errorf("aud and clientid do not match %s %s", res.AUD, config.GlobalMutableConfig.GetS("TwitchClientID"))
	}

	return true, p.VerifierID, nil
}

func NewTwitchVerifier() *TwitchVerifier {
	return &TwitchVerifier{
		Timeout: 60 * time.Second,
	}
}
