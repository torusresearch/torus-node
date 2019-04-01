package dkgnode

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"testing"
	"time"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/intel-go/fastjson"
	cache "github.com/patrickmn/go-cache"

	"github.com/stretchr/testify/assert"
)

type TestTransportStruct struct {
	Hex string
}

type TestDecodeStruct struct {
	I int
}

func TestDuplicateTokenCheck(t *testing.T) {
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
		"Torus00",
		"45cff8833a2139e63e474bda743e98b51b170e088a746610b98a06928f95a339",
		"e0db55130832d5fe303bd3e5a8e02e1f2a283095051a734820a536fc874edc34",
		"a96e89cf1a46ff67c8f7fbd7469d379a37495b2c539b6d2a4e9df9c45afb29f4",
		"1553777319",
		"google",
	}
	byt, _ := fastjson.Marshal(commitmentRequestParams)
	rawMessage := fastjson.RawMessage(byt)
	response, rpcErr := h.ServeJSONRPC(context.Background(), &rawMessage)
	if rpcErr != nil {
		assert.Fail(t, "Initial token request failed with error: "+rpcErr.Message)
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

	commitmentRequestParams = &CommitmentRequestParams{
		"mug00",
		"45cff8833a2139e63e474bda743e98b51b170e088a746610b98a06928f95a339",
		"e0db55130832d5fe303bd3e5a8e02e1f2a283095051a734820a536fc874edc34",
		"a96e89cf1a46ff67c8f7fbd7469d379a37495b2c539b6d2a4e9df9c45afb29f4",
		"1553777315",
		"google",
	}
	byt, _ = fastjson.Marshal(commitmentRequestParams)
	rawMessage = fastjson.RawMessage(byt)
	response, rpcErr = h.ServeJSONRPC(context.Background(), &rawMessage)
	if rpcErr == nil {
		assert.Fail(t, "Reusing token did not trigger and error response: "+rpcErr.Message)
	} else {
		assert.Equal(t, rpcErr.Data, "Duplicate token found")
	}
	privateKeyECDSA, _ := ethCrypto.HexToECDSA("16b4b2bc0103bfb9697dc52d7d795fe119daa0ea8adee71fae9c6549d1cc1da9")
	pubkey := privateKeyECDSA.Public().(*ecdsa.PublicKey)
	sig := ECDSASign([]byte(commitmentRequestResultData.ToString()), privateKeyECDSA)
	hexSig := ECDSASigToHex(sig)
	recSig := HexToECDSASig(hexSig)
	var sig32 [32]byte
	copy(sig32[:], secp256k1.Keccak256([]byte(commitmentRequestResultData.ToString()))[:32])
	recoveredSig := ECDSASignature{
		recSig.Raw,
		sig32,
		recSig.R,
		recSig.S,
		recSig.V - 27,
	}
	assert.True(t, ECDSAVerify(*pubkey, recoveredSig))
}
