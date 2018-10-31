package dkgnode

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
)

type AuthBodyGoogle struct {
	Azp   string `json:"azp"`
	Email string `json:"email"`
}

func testOauth(idToken string, email string) (*bool, error) {
	clientID := "876733105116-i0hj3s53qiio5k95prpfmj0hp0gmgtor.apps.googleusercontent.com"
	client := http.DefaultClient
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + idToken)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var body AuthBodyGoogle
	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, err
	}

	oAuthCorrect := true
	if strings.Compare(clientID, body.Azp) != 0 || strings.Compare(email, body.Email) != 0 {
		oAuthCorrect = false
	}
	return &oAuthCorrect, nil
}
