package pvss

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	xlabcommon "github.com/xlab-si/emmy/crypto/common"

	"github.com/torusresearch/torus-common/secp256k1"
)

func TestDLEQ(t *testing.T) {
	x := RandomBigInt()
	p, xG, xH := GenerateDLEQProof(secp256k1.G, secp256k1.H, *x)
	assert.True(t, p.VerifyDLEQProof(secp256k1.G, secp256k1.H, xG, xH))
}

/*
 * Copyright 2017 XLAB d.o.o.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

func TestCSPaillierEC(t *testing.T) {
	csp := NewCSPaillierEC(
		&CSPaillierECSecParams{
			L:        512,
			RoLength: 160,
			K:        158,
			K1:       158,
		})

	cspSec, _ := NewCSPaillierECFromSecKey(csp.SecKey)
	cspPub := NewCSPaillierECFromPubKey(csp.PubKey)

	m := xlabcommon.GetRandomInt(big.NewInt(8685849))
	label := xlabcommon.GetRandomInt(big.NewInt(340002223232))

	u, e, v, _ := cspPub.Encrypt(m, label)
	p, _ := cspSec.Decrypt(u, e, v, label)
	assert.Equal(t, m, p, "Camenisch-Shoup modified Paillier encryption/decryption does not work correctly")

	l, delta := cspPub.GetOpeningMsg(m)
	u1, e1, v1, delta1, l1, _ := cspPub.GetProofRandomData(u, e, label)

	cspVer := NewCSPaillierECFromPubKey(csp.PubKey)

	cspVer.SetVerifierEncData(u, e, v, *delta, label, l)
	challenge := cspVer.GetChallenge()
	cspVer.SetProofRandomData(u1, e1, v1, *delta1, l1, challenge)

	rTilde, sTilde, mTilde := cspPub.GetProofData(challenge)

	assert.True(t, cspVer.Verify(rTilde, sTilde, mTilde), "Camenisch-Shoup modified Paillier verifiable encryption proof does not work correctly")
	assert.True(t, new(big.Int).Abs(v).Cmp(v) == 0, "Camenisch-Shoup modified Paillier verifiable encryption proof does not work correctly")
}
