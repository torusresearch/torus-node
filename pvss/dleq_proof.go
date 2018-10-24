package pvss

import "math/big"

type PublicShare struct {
	Index int
	Value Point
}

type DLEQProof struct {
	c  big.Int
	r  big.Int
	vG Point
	vH Point
	xG Point
	xH Point
}

// NewDLEQProof computes a new NIZK dlog-equality proof for the scalar x with
// respect to base points G and H. It therefore randomly selects a commitment v
// and then computes the challenge c = H(xG,xH,vG,vH) and response r = v - cx.
// Besides the proof, this function also returns the encrypted base points xG
// and xH.
func getDLEQProof(secret big.Int, nodePubKey Point) *DLEQProof {
	//Encrypt bbase points with secret
	xG := pt(s.ScalarBaseMult(secret.Bytes()))
	xH := pt(s.ScalarMult(&nodePubKey.x, &nodePubKey.y, secret.Bytes()))

	// Commitment
	v := randomBigInt()
	vG := pt(s.ScalarBaseMult(v.Bytes()))
	vH := pt(s.ScalarMult(&nodePubKey.x, &nodePubKey.y, v.Bytes()))

	//Concat hashing bytes
	cb := make([]byte, 0)
	for _, element := range [4]Point{xG, xH, vG, vH} {
		cb = append(cb[:], element.x.Bytes()...)
		cb = append(cb[:], element.y.Bytes()...)
	}

	//hash
	hashed := Keccak256(cb)
	c := new(big.Int).SetBytes(hashed)
	c.Mod(c, generatorOrder)

	//response
	r := new(big.Int)
	r.Mul(c, &secret)
	r.Mod(r, generatorOrder)
	r.Sub(v, r) //do we need to mod here?
	r.Mod(r, generatorOrder)

	return &DLEQProof{*c, *r, vG, vH, xG, xH}
}

func batchGetDLEQProof(nodes []Point, shares []PrimaryShare) []*DLEQProof {
	if len(nodes) != len(shares) {
		return nil
	}
	proofs := make([]*DLEQProof, len(nodes))
	for i := range nodes {
		proofs[i] = getDLEQProof(shares[i].Value, nodes[i])
	}
	return proofs
}

// Verify examines the validity of the NIZK dlog-equality proof.
// The proof is valid if the following two conditions hold:
//   vG == rG + c(xG)
//   vH == rH + c(xH)
func verifyProof(proof DLEQProof, nodePubKey Point) bool {
	rGx, rGy := s.ScalarBaseMult(proof.r.Bytes())
	rHx, rHy := s.ScalarMult(&nodePubKey.x, &nodePubKey.y, proof.r.Bytes())
	cxGx, cxGy := s.ScalarMult(&proof.xG.x, &proof.xG.y, proof.c.Bytes())
	cxHx, cxHy := s.ScalarMult(&proof.xH.x, &proof.xH.y, proof.c.Bytes())
	ax, ay := s.Add(rGx, rGy, cxGx, cxGy)
	bx, by := s.Add(rHx, rHy, cxHx, cxHy)
	if !(proof.vG.x.Cmp(ax) == 0 && proof.vG.y.Cmp(ay) == 0 && proof.vH.x.Cmp(bx) == 0 && proof.vH.y.Cmp(by) == 0) {
		return false
	}
	return true
}

// DecryptShare first verifies the encrypted share against the encryption
// consistency proof and, if valid, decrypts it and creates a decryption
// consistency proof.
// func decShare(encShareOutputs EncShareOutputs, nodePubKey Point, nodePrivateKey big.Int) (*big.Int, error) {
// 	if err := verifyProof(encShareOutputs.Proof, nodePubKey); err != true {
// 		return nil, errors.New("share failed proof validation")
// 	}
// 	// G := suite.Point().Base()
// 	// V := suite.Point().Mul(suite.Scalar().Inv(x), encShare.S.V) // decryption: x^{-1} * (xS)
// 	invPrivKey := new(big.Int)
// 	invPrivKey.ModInverse(&nodePrivateKey, generatorOrder)
// 	shareGx, shareGy := s.ScalarMult(&encShareOutputs.EncryptedShare.Value.x, &encShareOutputs.EncryptedShare.Value.y, invPrivKey.Bytes())
// 	// g^ share
// 	// ps := &share.PubShare{I: encShare.S.I, V: V}
// 	// P, _, _, err := dleq.NewDLEQProof(suite, G, V, x)
// 	shareG := Point{shareGx, shareGy}
// 	proof := getDlEQProof(nodePrivateKey, shareG)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	// return &PubVerShare{*ps, *P}, nil
// 	return nil, nil
// }
