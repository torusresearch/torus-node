package auth

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
)

type TestVerifier struct {
	CorrectIDToken string
}

type TestVerifierParams struct {
	IDToken string `json:"idtoken"`
	ID      string `json:"id"`
}

// GetIdentifier - return identifier string for verifier
func (v *TestVerifier) GetIdentifier() string {
	return "test"
}

// CleanToken - ensure that incoming token conforms to strict format to prevent replay attacks
func (v *TestVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

// VerifyRequestIdentity - verifies identity of user based on their token
func (v *TestVerifier) VerifyRequestIdentity(jsonToken *bijson.RawMessage) (bool, string, error) {
	var p TestVerifierParams
	if err := bijson.Unmarshal(*jsonToken, &p); err != nil {
		return false, "", err
	}

	p.IDToken = v.CleanToken(p.IDToken)

	if p.IDToken != v.CorrectIDToken {
		return false, "", fmt.Errorf("Token is not %s", v.CorrectIDToken)
	}

	logging.WithField("test token time", strconv.FormatInt(time.Now().Add(-45*time.Second).Unix(), 10)).Debug()
	return true, p.ID, nil
}

// NewTestVerifier - Constructor for the default test verifier
func NewTestVerifier(correctIDToken string) *TestVerifier {
	return &TestVerifier{
		CorrectIDToken: correctIDToken,
	}
}
