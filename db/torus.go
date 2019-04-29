package db

import (
	"math/big"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/keygen"
)

// To store necessary shares and secrets
// type AVSSKeygenStorage interface {
// 	StoreKEYGENSecret(keyIndex big.Int, secret KEYGENSecrets) error
// 	StoreCompletedShare(keyIndex big.Int, si big.Int, siprime big.Int) error
// }

var completedShareKeyBytes = byte('c')

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

func (t *TorusLDB) StoreKEYGENSecret(keyIndex big.Int, secret keygen.KEYGENSecrets) error {
	keyIndexBytes := keyIndex.Bytes()
	marshalledSecret, err := bijson.Marshal(secret)
	if err != nil {
		return err
	}
	t.db.Set(keyIndexBytes, marshalledSecret)
	return nil
}

func (t *TorusLDB) RetrieveKEYGENSecret(keyIndex big.Int) (*keygen.KEYGENSecrets, error) {
	keyIndexBytes := keyIndex.Bytes()

	res := t.db.Get(keyIndexBytes)
	ks := &keygen.KEYGENSecrets{}
	err := bijson.Unmarshal(res, ks)
	if err != nil {
		return nil, err
	}
	return ks, nil

}

type completedShare struct {
	Si        big.Int      `json:"si"`
	SiPrime   big.Int      `json:"si_prime"`
	PublicKey common.Point `json:"public_key"`
}

func (c completedShare) MarshalBinary() ([]byte, error) {
	data, err := bijson.Marshal(c)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *completedShare) UnmarshalBinary(data []byte) error {
	err := bijson.Unmarshal(data, c)
	if err != nil {
		return err
	}

	return nil

}

func (t *TorusLDB) StoreCompletedShare(keyIndex big.Int, si big.Int, siprime big.Int, publicKey common.Point) error {
	keyIndexBytes := keyIndex.Bytes()
	marshalledShare, err := completedShare{
		Si:        si,
		SiPrime:   siprime,
		PublicKey: publicKey,
	}.MarshalBinary()
	if err != nil {
		return err
	}

	t.db.Set(keyIndexBytes, marshalledShare)
	return nil
}

func (t *TorusLDB) RetrieveCompletedShare(keyIndex big.Int) (*big.Int, *big.Int, *common.Point, error) {
	keyIndexBytes := keyIndex.Bytes()
	completedShareKey := append(keyIndexBytes, completedShareKeyBytes)

	res := t.db.Get(completedShareKey)
	var retrievedShare completedShare
	err := retrievedShare.UnmarshalBinary(res)
	if err != nil {
		return nil, nil, nil, err
	}

	return &retrievedShare.Si, &retrievedShare.SiPrime, &retrievedShare.PublicKey, nil
}
