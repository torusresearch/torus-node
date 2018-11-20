package pvss

import "math/big"

// This implements verifiable encryption from the Stadler
// Publicly Verifiable Secret Sharing paper

type DLEQProof struct {
	C  big.Int // challenge
	R  big.Int // response
	VG Point   // public commitment with respect to base point G
	VH Point   // public commitment with respect to base point H
}

func GenerateDLEQProof(G Point, H Point, x big.Int) (p *DLEQProof, xG Point, xH Point) {
	// encrypt base points with x
	xG = pt(s.ScalarBaseMult(x.Bytes()))
	xH = pt(s.ScalarMult(&H.X, &H.Y, x.Bytes()))

	// commitment
	v := RandomBigInt()
	vG := pt(s.ScalarBaseMult(v.Bytes()))
	vH := pt(s.ScalarMult(&H.X, &H.Y, v.Bytes()))

	// challenge: c = hash(xG, xH, vG, vH)
	tempBytes := make([]byte, 0)
	for _, element := range [4]Point{xG, xH, vG, vH} {
		tempBytes = append(tempBytes, element.X.Bytes()...)
		tempBytes = append(tempBytes, element.Y.Bytes()...)
	}
	hash := Keccak256(tempBytes)
	c := new(big.Int).SetBytes(hash)
	c.Mod(c, generatorOrder)

	// response: r = v - cx
	r := new(big.Int)
	r.Mul(&x, c)
	r.Sub(v, r)
	r.Mod(r, generatorOrder)

	p = &DLEQProof{
		C:  *c,
		R:  *r,
		VG: vG,
		VH: vH,
	}

	return
}

func (p *DLEQProof) Verify(G Point, H Point, xG Point, xH Point) bool {
	rG := pt(s.ScalarMult(&G.X, &G.Y, p.R.Bytes()))
	rH := pt(s.ScalarMult(&H.X, &H.Y, p.R.Bytes()))
	cxG := pt(s.ScalarMult(&xG.X, &xG.Y, p.C.Bytes()))
	cxH := pt(s.ScalarMult(&xH.X, &xH.Y, p.C.Bytes()))
	a := pt(s.Add(&rG.X, &rG.Y, &cxG.X, &cxG.Y))
	b := pt(s.Add(&rH.X, &rH.Y, &cxH.X, &cxH.Y))
	if !(p.VG.X.Cmp(&a.X) == 0 &&
		p.VG.Y.Cmp(&a.Y) == 0 &&
		p.VH.X.Cmp(&b.X) == 0 &&
		p.VH.Y.Cmp(&b.Y) == 0) {
		return false
	} else {
		return true
	}
}

type VerifiableProof struct {
}
