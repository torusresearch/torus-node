package dkgnode

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	core_types "github.com/tendermint/tendermint/rpc/core/types"

	"github.com/torusresearch/torus-public/mocks"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/torusresearch/torus-public/auth"
	"github.com/torusresearch/torus-public/secp256k1"

	cache "github.com/patrickmn/go-cache"
	"github.com/torusresearch/bijson"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type TestTransportStruct struct {
	Hex string
}

type TestDecodeStruct struct {
	I int
}

func TestCommitmentRequest(t *testing.T) {
	nodePrivK, _ := ecdsa.GenerateKey(secp256k1.Curve, rand.Reader)
	nodePubK := nodePrivK.Public().(*ecdsa.PublicKey)
	tokenCaches := make(map[string]*cache.Cache)
	var h = CommitmentRequestHandler{
		suite: &Suite{
			EthSuite: &EthSuite{
				NodePrivateKey: nodePrivK,
				NodePublicKey:  nodePubK,
			},
			CacheSuite: &CacheSuite{
				TokenCaches: tokenCaches,
			},
		},
		TimeNow: func() time.Time {
			return time.Unix(1553777319, 0)
		},
	}
	tokenCaches["google"] = cache.New(cache.NoExpiration, 10*time.Minute)
	// userprivkey 16b4b2bc0103bfb9697dc52d7d795fe119daa0ea8adee71fae9c6549d1cc1da9"
	// useraddress 0xf85ec6f642c30515913407e7d4dc1e6b88a86c31
	// idtoken "eyJhbGciOiJSUzI1NiIsImtpZCI6ImE0MzEzZTdmZDFlOWUyYTRkZWQzYjI5MmQyYTdmNGU1MTk1NzQzMDgiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJhY2NvdW50cy5nb29nbGUuY29tIiwiYXpwIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwiYXVkIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwic3ViIjoiMTEwMDc0Mjc5MTYxNzk0NDY1MTYxIiwiZW1haWwiOiJ0cm9uc2t5dHJvbGxAZ21haWwuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImF0X2hhc2giOiJUUXRsdlkwb2hSd1lfdm9qQ3lfSEpnIiwibmFtZSI6InRyb25za3l0cm9sbCIsInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vLUZzU2g1RXpVeHJNL0FBQUFBQUFBQUFJL0FBQUFBQUFBQUJ3L29CRkNSdWQ4X1JrL3M5Ni1jL3Bob3RvLmpwZyIsImdpdmVuX25hbWUiOiJ0cm9uc2t5dHJvbGwiLCJsb2NhbGUiOiJlbiIsImlhdCI6MTU1Mzc4MzY2NSwiZXhwIjoxNTUzNzg3MjY1LCJqdGkiOiIxNGQ0Njc5Yjg5M2ZiNjQzZDhkNDc0Y2Q4MjJkZDgwMzcwMTEzZjMwIn0.kAYUhG2diwNVYMgOUu3ZS-QDy62MDkKPk2jcPwdrcZWbF7w3rwGwrhgoAFiBmXwQ9Xbf8xXNGFGrF7M0Ixp7I6A5mJbwu69R0z2Ul9piyF5JYAS8nPEdHVJ5XbWnoEXrwppr1_JiIGTWrooglHp-EVbLFiMJHeZi0dTWJGJL0QvCSDlY227a8rGsTKC0xmT9YToybSS4dv5pgxQ5dUOfx7UX6XykMy2M_0Z1owqFy_ZMWd4qIAw-9u54tRJ3Vr6yOB3psdgNoJYxrzTGe55TF_9cAv3pqst-pxj_0Y91r7J5x7ZIrmCG-O_D7xtqzWY71nmUhyf2dNaIQiQcbJG1NQ"
	commitmentRequestParams := &CommitmentRequestParams{
		"mug00",
		"45cff8833a2139e63e474bda743e98b51b170e088a746610b98a06928f95a339",
		"e0db55130832d5fe303bd3e5a8e02e1f2a283095051a734820a536fc874edc34",
		"a96e89cf1a46ff67c8f7fbd7469d379a37495b2c539b6d2a4e9df9c45afb29f4",
		"1553777319",
		"google",
	}
	byt, _ := bijson.Marshal(commitmentRequestParams)
	rawMessage := bijson.RawMessage(byt)
	response, rpcErr := h.ServeJSONRPC(context.Background(), &rawMessage)
	if rpcErr != nil {
		assert.Fail(t, "Initial token request failed with error: "+rpcErr.Error())
	}
	commitmentRequestResult := response.(CommitmentRequestResult)
	var commitmentRequestResultData CommitmentRequestResultData
	commitmentRequestResultData.FromString(commitmentRequestResult.Data)
	assert.Equal(t, commitmentRequestResultData.MessagePrefix, commitmentRequestParams.MessagePrefix)
	assert.Equal(t, commitmentRequestResultData.TokenCommitment, commitmentRequestParams.TokenCommitment)
	assert.Equal(t, commitmentRequestResultData.TempPubX, commitmentRequestParams.TempPubX)
	assert.Equal(t, commitmentRequestResultData.TempPubY, commitmentRequestParams.TempPubY)
	assert.Equal(t, commitmentRequestResultData.Timestamp, commitmentRequestParams.Timestamp)
	assert.Equal(t, commitmentRequestResultData.VerifierIdentifier, commitmentRequestParams.VerifierIdentifier)

	assert.Equal(t, commitmentRequestResult.NodePubX, nodePubK.X.Text(16))
	assert.Equal(t, commitmentRequestResult.NodePubY, nodePubK.Y.Text(16))

	recSig := HexToECDSASig(commitmentRequestResult.Signature)
	var sig32 [32]byte
	copy(sig32[:], secp256k1.Keccak256([]byte(commitmentRequestResultData.ToString()))[:32])
	recoveredSig := ECDSASignature{
		recSig.Raw,
		sig32,
		recSig.R,
		recSig.S,
		recSig.V - 27,
	}
	assert.True(t, ECDSAVerify(*nodePubK, recoveredSig))

	response, rpcErr = h.ServeJSONRPC(context.Background(), &rawMessage)
	if rpcErr == nil {
		assert.Fail(t, "Reusing token did not trigger and error response: "+rpcErr.Message)
	} else {
		assert.Equal(t, rpcErr.Data, "Duplicate token found")
	}
}

func TestShareRequest(t *testing.T) {
	// First of all, we get responses from commitmentRequest
	ethSuite := EthSuite{}
	var nodeResponses [11]CommitmentRequestResult
	for i := 0; i < 11; i++ {
		nodePrivK, _ := ecdsa.GenerateKey(secp256k1.Curve, rand.Reader)
		nodePubK := nodePrivK.Public().(*ecdsa.PublicKey)
		ethSuite.NodeList = append(ethSuite.NodeList, &NodeReference{
			Index:     big.NewInt(int64(i + 1)),
			PublicKey: nodePubK,
		})
		tokenCaches := make(map[string]*cache.Cache)
		var h = CommitmentRequestHandler{
			suite: &Suite{
				EthSuite: &EthSuite{
					NodePrivateKey: nodePrivK,
					NodePublicKey:  nodePubK,
				},
				CacheSuite: &CacheSuite{
					CacheInstance: cache.New(cache.NoExpiration, 10*time.Minute),
					TokenCaches:   tokenCaches,
				},
			},
			TimeNow: func() time.Time {
				return time.Unix(1553777319, 0)
			},
		}
		tokenCaches["google"] = cache.New(cache.NoExpiration, 10*time.Minute)
		// userprivkey 16b4b2bc0103bfb9697dc52d7d795fe119daa0ea8adee71fae9c6549d1cc1da9"
		// useraddress 0xf85ec6f642c30515913407e7d4dc1e6b88a86c31
		// idtoken "eyJhbGciOiJSUzI1NiIsImtpZCI6ImE0MzEzZTdmZDFlOWUyYTRkZWQzYjI5MmQyYTdmNGU1MTk1NzQzMDgiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJhY2NvdW50cy5nb29nbGUuY29tIiwiYXpwIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwiYXVkIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwic3ViIjoiMTEwMDc0Mjc5MTYxNzk0NDY1MTYxIiwiZW1haWwiOiJ0cm9uc2t5dHJvbGxAZ21haWwuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImF0X2hhc2giOiJUUXRsdlkwb2hSd1lfdm9qQ3lfSEpnIiwibmFtZSI6InRyb25za3l0cm9sbCIsInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vLUZzU2g1RXpVeHJNL0FBQUFBQUFBQUFJL0FBQUFBQUFBQUJ3L29CRkNSdWQ4X1JrL3M5Ni1jL3Bob3RvLmpwZyIsImdpdmVuX25hbWUiOiJ0cm9uc2t5dHJvbGwiLCJsb2NhbGUiOiJlbiIsImlhdCI6MTU1Mzc4MzY2NSwiZXhwIjoxNTUzNzg3MjY1LCJqdGkiOiIxNGQ0Njc5Yjg5M2ZiNjQzZDhkNDc0Y2Q4MjJkZDgwMzcwMTEzZjMwIn0.kAYUhG2diwNVYMgOUu3ZS-QDy62MDkKPk2jcPwdrcZWbF7w3rwGwrhgoAFiBmXwQ9Xbf8xXNGFGrF7M0Ixp7I6A5mJbwu69R0z2Ul9piyF5JYAS8nPEdHVJ5XbWnoEXrwppr1_JiIGTWrooglHp-EVbLFiMJHeZi0dTWJGJL0QvCSDlY227a8rGsTKC0xmT9YToybSS4dv5pgxQ5dUOfx7UX6XykMy2M_0Z1owqFy_ZMWd4qIAw-9u54tRJ3Vr6yOB3psdgNoJYxrzTGe55TF_9cAv3pqst-pxj_0Y91r7J5x7ZIrmCG-O_D7xtqzWY71nmUhyf2dNaIQiQcbJG1NQ"
		commitmentRequestParams := &CommitmentRequestParams{
			"mug00",
			"45cff8833a2139e63e474bda743e98b51b170e088a746610b98a06928f95a339",
			"e0db55130832d5fe303bd3e5a8e02e1f2a283095051a734820a536fc874edc34",
			"a96e89cf1a46ff67c8f7fbd7469d379a37495b2c539b6d2a4e9df9c45afb29f4",
			// "1553777319",
			strconv.Itoa(int(time.Now().Unix())),
			"google",
		}
		byt, _ := bijson.Marshal(commitmentRequestParams)
		rawMessage := bijson.RawMessage(byt)
		response, rpcErr := h.ServeJSONRPC(context.Background(), &rawMessage)
		if rpcErr != nil {
			assert.Fail(t, "Initial token request failed with error: "+rpcErr.Error())
		}
		commitmentRequestResult := response.(CommitmentRequestResult)
		nodeResponses[i] = commitmentRequestResult
	}

	mockClient := &mocks.Client{}
	mockClient.On("ABCIQuery", "GetEmailIndex", mock.Anything).Return(&core_types.ResultABCIQuery{
		Response: abci.ResponseQuery{
			Value: []byte("1"),
		},
	}, nil)
	// bundle collated node responses and send it to a node to get back your share
	userSecretShare := big.NewInt(int64(42))
	siStore := make(map[int]SiStore)
	siStore[1] = SiStore{
		Index: 1,
		Value: userSecretShare,
	}

	h := ShareRequestHandler{
		TimeNow: time.Now,
		suite: &Suite{
			Config: &Config{
				Threshold: 11,
			},
			DefaultVerifier: TestMockDefaultVerifier{},
			EthSuite:        &ethSuite,
			CacheSuite: &CacheSuite{
				CacheInstance: cache.New(cache.NoExpiration, 10*time.Minute),
			},
			BftSuite: &BftSuite{
				BftRPC: &BftRPC{
					mockClient,
				},
			},
		},
	}
	h.suite.CacheSuite.CacheInstance.Set("Si_MAPPING", siStore, 10*time.Minute)

	var nodeSignatures []NodeSignature
	for i := 0; i < len(nodeResponses); i++ {
		nodeResponse := nodeResponses[i]
		nodeSignatures = append(nodeSignatures, NodeSignature{
			nodeResponse.Signature,
			nodeResponse.Data,
			nodeResponse.NodePubX,
			nodeResponse.NodePubY,
		})
	}

	req := ShareRequestParams{
		ID:                 "test@tor.us",
		Token:              "eyJhbGciOiJSUzI1NiIsImtpZCI6ImE0MzEzZTdmZDFlOWUyYTRkZWQzYjI5MmQyYTdmNGU1MTk1NzQzMDgiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJhY2NvdW50cy5nb29nbGUuY29tIiwiYXpwIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwiYXVkIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwic3ViIjoiMTEwMDc0Mjc5MTYxNzk0NDY1MTYxIiwiZW1haWwiOiJ0cm9uc2t5dHJvbGxAZ21haWwuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImF0X2hhc2giOiJUUXRsdlkwb2hSd1lfdm9qQ3lfSEpnIiwibmFtZSI6InRyb25za3l0cm9sbCIsInBpY3R1cmUiOiJodHRwczovL2xoMy5nb29nbGV1c2VyY29udGVudC5jb20vLUZzU2g1RXpVeHJNL0FBQUFBQUFBQUFJL0FBQUFBQUFBQUJ3L29CRkNSdWQ4X1JrL3M5Ni1jL3Bob3RvLmpwZyIsImdpdmVuX25hbWUiOiJ0cm9uc2t5dHJvbGwiLCJsb2NhbGUiOiJlbiIsImlhdCI6MTU1Mzc4MzY2NSwiZXhwIjoxNTUzNzg3MjY1LCJqdGkiOiIxNGQ0Njc5Yjg5M2ZiNjQzZDhkNDc0Y2Q4MjJkZDgwMzcwMTEzZjMwIn0.kAYUhG2diwNVYMgOUu3ZS-QDy62MDkKPk2jcPwdrcZWbF7w3rwGwrhgoAFiBmXwQ9Xbf8xXNGFGrF7M0Ixp7I6A5mJbwu69R0z2Ul9piyF5JYAS8nPEdHVJ5XbWnoEXrwppr1_JiIGTWrooglHp-EVbLFiMJHeZi0dTWJGJL0QvCSDlY227a8rGsTKC0xmT9YToybSS4dv5pgxQ5dUOfx7UX6XykMy2M_0Z1owqFy_ZMWd4qIAw-9u54tRJ3Vr6yOB3psdgNoJYxrzTGe55TF_9cAv3pqst-pxj_0Y91r7J5x7ZIrmCG-O_D7xtqzWY71nmUhyf2dNaIQiQcbJG1NQ",
		NodeSignatures:     nodeSignatures,
		VerifierIdentifier: "testmockverifier",
	}
	byt, _ := bijson.Marshal(req)
	reqJSON := bijson.RawMessage(byt)
	response, err := h.ServeJSONRPC(context.Background(), &reqJSON)
	if err != nil {
		fmt.Println(err)
	}
	shareRequestResult := response.(ShareRequestResult)
	assert.Equal(t, shareRequestResult.HexShare, userSecretShare.Text(16))
}

type TestMockDefaultVerifier struct {
}

func (TestMockDefaultVerifier) Verify(*bijson.RawMessage) (bool, error) {
	return true, nil
}
func (t TestMockDefaultVerifier) Lookup(string) (auth.Verifier, error) {
	return TestMockVerifier{}, nil
}

type TestMockVerifier struct {
}

func (TestMockVerifier) CleanToken(s string) string {
	return s
}
func (TestMockVerifier) GetIdentifier() string {
	return "testmockverifier"
}
func (TestMockVerifier) VerifyRequestIdentity(*bijson.RawMessage) (bool, error) {
	return true, nil
}

type TestMockClient interface {
}
