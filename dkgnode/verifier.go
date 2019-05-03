package dkgnode

import (
	"github.com/torusresearch/torus-public/auth"
)

func SetupVerifier(suite *Suite) {
	// We can use a flag here to change the default verifier
	// In the future we should allow a range of verifiers
	suite.DefaultVerifier = auth.NewGeneralVerifier(googleIdentityVerifier{
		auth.NewDefaultGoogleVerifier(suite.Config.GoogleClientID),
		suite,
	})
}
