package dkgnode

import (
	"github.com/torusresearch/torus-public/auth"
)

const localTestVerifierKey = "notBluBlu"

func loadTestingSuite() *Suite {
	cfg := loadConfig("../config/config.local.1.json")
	//Main suite of functions used in node
	suite := Suite{}
	suite.Config = cfg
	suite.DefaultVerifier = auth.NewGeneralVerifier(
		&auth.DemoVerifier{
			ExpectedKey: localTestVerifierKey,
			Store:       make(map[string]bool),
		},
	)
	return &suite
}

// TODO: Make the test pass without triggering panic 'flag redefined: register', when it is run with other tests
// func TestBasicServerSetup(t *testing.T) {
// 	suite := loadTestingSuite()
// 	port := "3456"
// 	server := setUpServer(suite, port)
// 	go func() {
// 		err := server.ListenAndServe()
// 		if err != nil {
// 			t.Fatal(err)
// 		}

// 	}()

// 	client := &http.Client{}
// 	getURL := fmt.Sprintf("http://localhost:%s/healthz", port)
// 	resp, err := client.Get(getURL)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	if resp.StatusCode != 200 {
// 		t.Fatalf("server returned status %d but expected %d", resp.StatusCode, 200)
// 	}

// 	server.Shutdown(context.Background())

// }
