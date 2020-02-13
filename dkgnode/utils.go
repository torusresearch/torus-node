package dkgnode

import (
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"reflect"
	"runtime/debug"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethMath "github.com/ethereum/go-ethereum/common/math"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	logging "github.com/sirupsen/logrus"
	"github.com/struCoder/pidusage"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/idmutex"
)

func init() {
	go func() {
		interval := time.NewTicker(time.Second)
		for range interval.C {
			sysInfo, err := pidusage.GetStat(os.Getpid())
			if err != nil {
				logging.WithError(err).Error("could not get sysInfo")
			}
			sysInfoCache.Lock()
			logging.WithField("memory", sysInfo.Memory).Debug("updating sysInfoCache")
			sysInfoCache.Memory = *big.NewInt(int64(sysInfo.Memory))
			sysInfoCache.Unlock()
		}
	}()
}

var sysInfoCache = struct {
	idmutex.Mutex
	Memory big.Int
}{}

func stringify(i interface{}) string {
	bytArr, ok := i.([]byte)
	if ok {
		return string(bytArr)
	}
	str, ok := i.(string)
	if ok {
		return str
	}
	byt, err := bijson.Marshal(i)
	if err != nil {
		logging.WithError(err).Error("Could not bijsonmarshal")
	}
	return string(byt)
}

func createDirectory(dirName string) bool {
	src, err := os.Stat(dirName)

	if os.IsNotExist(err) {
		errDir := os.MkdirAll(dirName, 0755)
		if errDir != nil {
			panic(err)
		}
		return true
	}

	if src.Mode().IsRegular() {
		logging.WithField("dirName", dirName).Debug("directory already exist as a file!")
		return false
	}
	return false
}

func GetPeerIDFromP2pListenAddress(p2pListenAddress string) (*peer.ID, error) {
	ipfsaddr, err := ma.NewMultiaddr(p2pListenAddress)
	if err != nil {
		logging.WithError(err).Error("could not get ipfsaddr")
		return nil, err
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		logging.WithError(err).Error("could not get pid")
		return nil, err
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		logging.WithError(err).Error("could not get peerid")
		return nil, err
	}

	return &peerid, nil
}

func PointsArrayToBytesArray(pointsArray *[]common.Point) []byte {
	arrBytes := []byte{}
	for _, item := range *pointsArray {
		num := abi.U256(&item.X) // this has length of 32
		arrBytes = append(arrBytes, num...)
		num = abi.U256(&item.Y)
		arrBytes = append(arrBytes, num...)
	}
	return arrBytes
}

func BytesArrayToPointsArray(byteArray []byte) (pointsArray []*common.Point) {
	if len(byteArray)%32 > 0 {
		logging.WithField("byteArray", byteArray).Debug("error with data, not an array of U256s")
		return
	}
	bigIntArray := make([]*big.Int, len(byteArray)/32)
	for index := 0; index < len(byteArray)/32; index++ {
		bigInt, ok := ethMath.ParseBig256("0x" + hex.EncodeToString(byteArray[index*32:index*32+32]))
		if !ok {
			logging.WithField("data", byteArray[index*32:index*32+32]).Debug("Error with data, could not parse big256")
			return
		}
		bigIntArray[index] = bigInt
	}

	if len(bigIntArray)%2 == 1 {
		logging.WithField("bigIntArray", bigIntArray).Debug("Error with data, not an even number of bigInts")
		return
	}

	for ind := 0; ind < len(bigIntArray); ind = ind + 2 {
		if bigIntArray[ind] != nil && bigIntArray[ind+1] != nil {
			pointsArray = append(pointsArray, &common.Point{X: *bigIntArray[ind], Y: *bigIntArray[ind+1]})
		} else {
			logging.Debug("error fatal, bigIntArray is malformed")
		}
	}
	return
}

// HashToString hashes the first 6 chars to a hex string
func HashToString(bytes []byte) string {
	hash := secp256k1.Keccak256(bytes)
	return hex.EncodeToString(hash)[0:6]
}

// incrementLastBit - used to delimit the range of elements in the db that were stored based on this prefix
func incrementLastBit(b []byte) []byte {
	end := new(big.Int).SetBytes(b)
	end = end.Add(end, big.NewInt(1))
	return end.Bytes()
}

type EventBusBytes []byte

// castOrUnmarshal allows us to pass in any interface and it will either cast dataInter to v or unmarshal bytes to v
// it does so by detecting if dataInter is of a custom bytes type
// accepts a silent flag, which skips all logging and debugging (may be expensive)
func castOrUnmarshal(dataInter interface{}, v interface{}, flags ...bool) (err error) {
	var silent bool
	if len(flags) >= 1 && flags[0] {
		silent = true
	}
	defer func() {
		if r := recover(); r != nil {
			if !silent {
				logging.WithField("recover", r).WithField("stack", string(debug.Stack())).Debug("could not cast in castOrUnmarshal")
			}
			err = errors.New("could not cast in castOrUnmarshal")
		}
	}()
	data, ok := dataInter.(EventBusBytes)
	if ok {
		err = bijson.Unmarshal(data, v)
		if err != nil {
			logging.WithField("data", data).WithError(err).Debug("could not unmarshal in castOrUnmarshal")
		}
	} else {
		if reflect.ValueOf(dataInter).Kind() == reflect.Ptr {
			reflect.ValueOf(v).Elem().Set(reflect.ValueOf(dataInter).Elem())
		} else {
			reflect.ValueOf(v).Elem().Set(reflect.ValueOf(dataInter))
		}
	}
	return
}

func getRSSMemory() big.Int {
	sysInfoCache.Lock()
	defer sysInfoCache.Unlock()
	return sysInfoCache.Memory
}
