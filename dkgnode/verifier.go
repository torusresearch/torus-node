package dkgnode

import (
	"github.com/torusresearch/torus-public/auth"
)

func SetupVerifier(suite *Suite) {
	// We can use a flag here to change the default verifier
	// In the future we should allow a range of verifiers
	var verfier auth.GeneralVerifier
	if suite.Config.IsDebug {
		verfier = auth.NewGeneralVerifier(
			auth.NewTestVerifier("blublu"),
			googleIdentityVerifier{
				auth.NewDefaultGoogleVerifier(suite.Config.GoogleClientID),
				suite,
			})
	} else {
		verfier = auth.NewGeneralVerifier(googleIdentityVerifier{
			auth.NewDefaultGoogleVerifier(suite.Config.GoogleClientID),
			suite,
		})
	}
	suite.DefaultVerifier = verfier
}
