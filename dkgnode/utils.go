package dkgnode

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/secp256k1"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethMath "github.com/ethereum/go-ethereum/common/math"
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
	// to get data []byte from string, do secp256k1.Keccak256([]byte(messageString))
	hashRaw := secp256k1.Keccak256(data)
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

// [X1, Y1, X2, Y2, X3, Y3 ... ]
func PointsArrayToBytesArray(pointsArray *[]common.Point) []byte {
	arrBytes := []byte{}
	for _, item := range *pointsArray {
		var num []byte
		num = abi.U256(&item.X) // this has length of 32
		// fmt.Println(len(num))
		arrBytes = append(arrBytes, num...)
		num = abi.U256(&item.Y)
		arrBytes = append(arrBytes, num...)
	}
	return arrBytes
}

func BytesArrayToPointsArray(byteArray []byte) (pointsArray []*common.Point) {
	if len(byteArray)%32 > 0 {
		fmt.Println("Error with data, not an array of U256s")
		fmt.Println(len(byteArray), byteArray)
		return
	}
	bigIntArray := make([]*big.Int, len(byteArray)/32)
	for index := 0; index < len(byteArray)/32; index++ {
		bigInt, ok := ethMath.ParseBig256("0x" + hex.EncodeToString(byteArray[index*32:index*32+32]))
		if !ok {
			fmt.Println("Error with data, could not parse big256")
			fmt.Println(byteArray[index*32 : index*32+32])
			return
		}
		bigIntArray[index] = bigInt
	}

	if len(bigIntArray)%2 == 1 {
		fmt.Println("Error with data, not an even number of bigInts")
		fmt.Println(bigIntArray)
		return
	}

	for ind := 0; ind < len(bigIntArray); ind = ind + 2 {
		if bigIntArray[ind] != nil && bigIntArray[ind+1] != nil {
			pointsArray = append(pointsArray, &common.Point{X: *bigIntArray[ind], Y: *bigIntArray[ind+1]})
		} else {
			fmt.Println("Error fatal, bigIntArray is malformed")
		}
	}
	return
}
