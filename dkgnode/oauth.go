package dkgnode

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/torusresearch/torus-public/logging"
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

func testOauth(suite *Suite, idToken string, email string) (bool, error) {
	clientID := "876733105116-i0hj3s53qiio5k95prpfmj0hp0gmgtor.apps.googleusercontent.com"
	client := http.DefaultClient
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + idToken)
	if err != nil {
		return false, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var body AuthBodyGoogle
	err = json.Unmarshal(b, &body)
	if err != nil {
		return false, err
	}

	// oAuthCorrect := true
	// Check if auth token has been signed within declared parameter
	//TODO: config var?? AND how do we decide on this parameter
	//TODO: should we return auth errors for developers get in jrpc response
	timeSignedInt, err := strconv.Atoi(body.Iat)
	if err != nil {
		// QUESTION(TEAM) - unhandled error, was only fmt.Printlnd
		// since there is a todo at the top. maybe worth looking at
		logging.Error(err.Error())
	}

	timeSigned := time.Unix(int64(timeSignedInt), 0)
	if timeSigned.Add(60 * time.Second).Before(time.Now()) {
		return false, errors.New("timesigned is more than 60 seconds ago " + timeSigned.String())
	}

	if strings.Compare(clientID, body.Azp) != 0 {
		return false, errors.New("azip is not clientID " + body.Azp + " " + clientID)
	}

	if strings.Compare(email, body.Email) != 0 {
		return false, errors.New("email not equal to body.email " + email + " " + body.Email)
	}

	//check if oauth is in cache
	_, ok := suite.CacheSuite.OAuthCacheInstance.Get(idToken)
	if ok {
		return false, errors.New("oauth is already in cache " + idToken)
	}

	//add token to cache should clean every 5 seconds
	suite.CacheSuite.OAuthCacheInstance.Set(idToken, true, 0)

	return true, nil
}
