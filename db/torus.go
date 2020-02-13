package db

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	logging "github.com/sirupsen/logrus"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	pcmn "github.com/torusresearch/torus-node/common"
)

var completedKeygenShareBytes = []byte("a")
var completedPSSShareBytes = []byte("b")
var completedShareCountBytes = []byte("c")
var keygenCommitmentMatrixBytes = []byte("d")
var pssCommitmentMatrixBytes = []byte("e")
var pubkeyToKeyIndexBytes = []byte("f")
var keygenIDBytes = []byte("g")
var keyIndexToPubKeyBytes = []byte("h")
var connectionDetailsBytes = []byte("i")
var nodePubKeyBytes = []byte("j")

// TorusLDB implements TorusDB on top of LevelDB
type TorusLDB struct {
	db DB
}

// NewTorusLDB returns a leveldb implementation of TorusDB
// NOTE: dbDirPath MUST be a directory
func NewTorusLDB(dbDirPath string) (*TorusLDB, error) {
	db, err := NewGoLevelDB(dbDirPath)
	if err != nil {
		return nil, err
	}
	return &TorusLDB{
		db: db,
	}, nil
}

type completedShare struct {
	Si      big.Int `json:"si"`
	SiPrime big.Int `json:"si_prime"`
}

func (t *TorusLDB) StoreNodePubKey(nodeAddress ethCommon.Address, pubKey common.Point) error {
	key := append(nodePubKeyBytes, nodeAddress[:]...)
	data, err := bijson.Marshal(pubKey)
	if err != nil {
		return err
	}
	t.db.Set(key, data)
	return nil
}

func (t *TorusLDB) RetrieveNodePubKey(nodeAddress ethCommon.Address) (pubKey common.Point, err error) {
	key := append(nodePubKeyBytes, nodeAddress[:]...)
	data := t.db.Get(key)
	if data == nil {
		return pubKey, fmt.Errorf("could not find pubkey for nodeAddress %s", nodeAddress.String())
	}
	err = bijson.Unmarshal(data, &pubKey)
	return
}

func (t *TorusLDB) StoreConnectionDetails(nodeAddress ethCommon.Address, tmP2PConnection string, p2pConnection string) error {
	connectionDetailsKey := append(connectionDetailsBytes, nodeAddress[:]...)
	connectionData := strings.Join([]string{tmP2PConnection, p2pConnection}, pcmn.Delimiter1)
	t.db.Set(connectionDetailsKey, []byte(connectionData))
	return nil
}

func (t *TorusLDB) RetrieveConnectionDetails(nodeAddress ethCommon.Address) (tmP2PConnection string, p2pConnection string, err error) {
	connectionDetailsKey := append(connectionDetailsBytes, nodeAddress[:]...)
	res := t.db.Get(connectionDetailsKey)
	if res != nil {
		substrs := strings.Split(string(res), pcmn.Delimiter1)
		if len(substrs) != 2 {
			return "", "", errors.New("unexpected number of substrs in connection details stored data")
		}
		tmP2PConnection = substrs[0]
		p2pConnection = substrs[1]
		return
	}
	return "", "", errors.New("could not get data from db for connection details")
}

func (t *TorusLDB) StoreKeygenCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error {
	keyIndexBytes := keyIndex.Bytes()
	commitmentMatrixKey := append(keygenCommitmentMatrixBytes, keyIndexBytes...)
	b, err := bijson.Marshal(c)
	if err != nil {
		logging.WithField("c", c).WithField("keyIndex", keyIndex).Debug("could not store commitment matrix")
		return err
	}
	t.db.Set(commitmentMatrixKey, b)
	return nil
}

func (t *TorusLDB) StorePSSCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error {
	keyIndexBytes := keyIndex.Bytes()
	commitmentMatrixKey := append(pssCommitmentMatrixBytes, keyIndexBytes...)
	b, err := bijson.Marshal(c)
	if err != nil {
		logging.WithField("c", c).WithField("keyIndex", keyIndex).Debug("could not store commitment matrix")
		return err
	}
	t.db.Set(commitmentMatrixKey, b)
	return nil
}

func (t *TorusLDB) RetrieveCommitmentMatrix(keyIndex big.Int) ([][]common.Point, error) {
	keyIndexBytes := keyIndex.Bytes()

	pssCommitmentMatrixKey := append(pssCommitmentMatrixBytes, keyIndexBytes...)
	res := t.db.Get(pssCommitmentMatrixKey)
	if res != nil {
		var retrievedCommitmentMatrix [][]common.Point
		err := bijson.Unmarshal(res, &retrievedCommitmentMatrix)
		if err != nil {
			logging.WithField("res", res).WithField("keyIndex", keyIndex).Debug("could not retrieve pss commitment matrix")
			return nil, err
		}
		return retrievedCommitmentMatrix, nil
	}

	keygenCommitmentMatrixKey := append(keygenCommitmentMatrixBytes, keyIndexBytes...)
	res = t.db.Get(keygenCommitmentMatrixKey)
	if res != nil {
		var retrievedCommitmentMatrix [][]common.Point
		err := bijson.Unmarshal(res, &retrievedCommitmentMatrix)
		if err != nil {
			logging.WithField("res", res).WithField("keyIndex", keyIndex).Debug("could not retrieve keygen commitment matrix")
			return nil, err
		}
		return retrievedCommitmentMatrix, nil
	}

	return nil, nil
}

func (t *TorusLDB) StorePublicKeyToKeyIndex(publicKey common.Point, keyIndex big.Int) error {
	b, err := bijson.Marshal(publicKey)
	if err != nil {
		return err
	}
	// store pubkey -> key index
	pkkey := append(pubkeyToKeyIndexBytes, b...)
	t.db.Set(pkkey, keyIndex.Bytes())

	// store key index -> pubkey
	kikey := append(keyIndexToPubKeyBytes, keyIndex.Bytes()...)
	t.db.Set(kikey, b)

	return nil
}

func (t *TorusLDB) RetrievePublicKeyToKeyIndex(publicKey common.Point) (*big.Int, error) {
	b, err := bijson.Marshal(publicKey)
	if err != nil {
		return nil, err
	}
	key := append(pubkeyToKeyIndexBytes, b...)
	var keyIndex big.Int
	keyIndexBytes := t.db.Get(key)
	keyIndex.SetBytes(keyIndexBytes)
	return &keyIndex, nil
}

func (t *TorusLDB) RetrieveKeyIndexToPublicKey(keyIndex big.Int) (*common.Point, error) {
	key := append(keyIndexToPubKeyBytes, keyIndex.Bytes()...)
	pubKeyBytes := t.db.Get(key)
	var pubKey common.Point
	err := bijson.Unmarshal(pubKeyBytes, &pubKey)
	if err != nil {
		return nil, err
	}
	return &pubKey, nil
}

func (t *TorusLDB) KeyIndexToPublicKeyExists(keyIndex big.Int) bool {
	key := append(keyIndexToPubKeyBytes, keyIndex.Bytes()...)
	return t.db.Has(key)
}

func (t *TorusLDB) StoreCompletedKeygenShare(keyIndex big.Int, si big.Int, siprime big.Int) error {
	keyIndexBytes := keyIndex.Bytes()
	completedShareKey := append(completedKeygenShareBytes, keyIndexBytes...)
	marshalledShare, err := bijson.Marshal(completedShare{
		Si:      si,
		SiPrime: siprime,
	})
	if err != nil {
		return err
	}

	t.db.Set(completedShareKey, marshalledShare)
	t.db.Set(append(completedShareCountBytes, keyIndexBytes...), []byte("1"))
	return nil
}

func (t *TorusLDB) StoreCompletedPSSShare(keyIndex big.Int, si big.Int, siprime big.Int) error {
	keyIndexBytes := keyIndex.Bytes()
	completedShareKey := append(completedPSSShareBytes, keyIndexBytes...)
	marshalledShare, err := bijson.Marshal(completedShare{
		Si:      si,
		SiPrime: siprime,
	})
	if err != nil {
		return err
	}
	t.db.Set(completedShareKey, marshalledShare)
	t.db.Set(append(completedShareCountBytes, keyIndexBytes...), []byte("1"))
	return nil
}

func (t *TorusLDB) RetrieveCompletedShare(keyIndex big.Int) (*big.Int, *big.Int, error) {
	keyIndexBytes := keyIndex.Bytes()
	completedPSSShareKey := append(completedPSSShareBytes, keyIndexBytes...)
	var res []byte
	res = t.db.Get(completedPSSShareKey)
	if res != nil {
		var retrievedShare completedShare
		err := bijson.Unmarshal(res, &retrievedShare)
		if err != nil {
			return nil, nil, err
		}
		return &retrievedShare.Si, &retrievedShare.SiPrime, nil
	}
	completedKeygenShareKey := append(completedKeygenShareBytes, keyIndexBytes...)
	res = t.db.Get(completedKeygenShareKey)
	if res != nil {
		var retrievedShare completedShare
		err := bijson.Unmarshal(res, &retrievedShare)
		if err != nil {
			return nil, nil, err
		}
		return &retrievedShare.Si, &retrievedShare.SiPrime, nil
	}
	return nil, nil, nil
}

func (t *TorusLDB) GetShareCount() int {
	var endBytes = []byte{completedShareCountBytes[0] + 1}
	itr := t.db.Iterator(completedShareCountBytes, endBytes)
	count := 0
	for ; itr.Valid(); itr.Next() {
		count++
	}
	return count
}

type KeygenStarted struct {
	Started bool
}

func (t *TorusLDB) SetKeygenStarted(keygenID string, started bool) {
	key := append(keygenIDBytes, []byte(keygenID)...)
	data, err := bijson.Marshal(KeygenStarted{Started: started})
	if err != nil {
		logging.WithError(err).Error("Could not marshal set keygen started")
	}
	t.db.Set(key, data)
}
func (t *TorusLDB) GetKeygenStarted(keygenID string) bool {
	key := append(keygenIDBytes, []byte(keygenID)...)
	data := t.db.Get(key)
	if data == nil {
		return false
	}
	var keygenStarted KeygenStarted
	err := bijson.Unmarshal(data, &keygenStarted)
	if err != nil {
		logging.WithError(err).Error("Could not unmarshal get keygen started")
	}
	return keygenStarted.Started
}
