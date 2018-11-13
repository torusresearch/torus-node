package dkgnode

import (
	"crypto/ecdsa"
	"errors"
	"log"
	"math/big"
	"net"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
)

type ECDSASignature struct {
	Raw  []byte
	Hash [32]byte
	R    [32]byte
	S    [32]byte
	V    uint8
}

func findExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

func bytes32(bytes []byte) [32]byte {
	tmp := [32]byte{}
	copy(tmp[:], bytes)
	return tmp
}

func ECDSASign(data []byte, ecdsaKey *ecdsa.PrivateKey) ECDSASignature {
	// to get data []byte from string, do ethCrypto.Keccak256([]byte(messageString))
	hashRaw := ethCrypto.Keccak256(data)
	signature, err := ethCrypto.Sign(hashRaw, ecdsaKey)
	if err != nil {
		log.Fatal(err)
	}
	return ECDSASignature{
		signature,
		bytes32(hashRaw),
		bytes32(signature[:32]),
		bytes32(signature[32:64]),
		uint8(int(signature[64])) + 27, // Yes add 27, weird Ethereum quirk
	}
}

// func ECDSAValidateRaw(ecdsaPubBytes []byte, messageHash []byte, signature []byte) bool {
// 	return ethCrypto.VerifySignature(ecdsaPubBytes, messageHash, signature)
// }

func ECDSAVerify(ecdsaPubKey ecdsa.PublicKey, ecdsaSignature ECDSASignature) bool {
	r := new(big.Int)
	s := new(big.Int)
	r.SetBytes(ecdsaSignature.R[:])
	s.SetBytes(ecdsaSignature.S[:])

	return ecdsa.Verify(
		&ecdsaPubKey,
		ecdsaSignature.Hash[:],
		r,
		s,
	)
}
