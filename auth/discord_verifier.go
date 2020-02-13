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

type DiscordAuthResponse struct {
	Application struct {
		ID string `json:"id"`
	} `json:"application"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Expires string `json:"expires"`
}

type DiscordVerifier struct {
	Timeout time.Duration
}

type DiscordVerifierParams struct {
	IDToken    string `json:"idtoken"`
	VerifierID string `json:"verifier_id"`
}

func (d *DiscordVerifier) GetIdentifier() string {
	return "discord"
}

func (d *DiscordVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

func (d *DiscordVerifier) VerifyRequestIdentity(rawPayload *bijson.RawMessage) (bool, string, error) {
	var p DiscordVerifierParams
	if err := bijson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	req, err := http.NewRequest("GET", "https://discordapp.com/api/oauth2/@me", nil)
	if err != nil {
		return false, "", err
	}

	req.Header.Set("User-Agent", "DiscordBot (https://app.tor.us/, v0.1.2)")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.IDToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}

	var res DiscordAuthResponse
	if err := bijson.Unmarshal(body, &res); err != nil {
		return false, "", err
	}

	timeExpires, err := time.Parse(time.RFC3339, res.Expires)
	if err != nil {
		return false, "", err
	}
	timeSigned := timeExpires.Add(-7 * 24 * time.Hour) // subtract 7 days from expiry
	if timeSigned.Add(d.Timeout).Before(time.Now()) {
		return false, "", errors.New("timesigned is more than 60 seconds ago " + timeSigned.String())
	}

	if p.VerifierID != res.User.ID {
		return false, "", fmt.Errorf("IDs do not match %s %s", p.VerifierID, res.User.ID)
	}

	if config.GlobalMutableConfig.GetS("DiscordClientID") != res.Application.ID {
		return false, "", fmt.Errorf("ClientIDs do not match %s %s", config.GlobalMutableConfig.GetS("DiscordClientID"), res.Application.ID)
	}

	return true, p.VerifierID, nil
}

func NewDiscordVerifier() *DiscordVerifier {
	return &DiscordVerifier{
		Timeout: 60 * time.Second,
	}
}
