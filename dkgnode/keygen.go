package dkgnode

/* All useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	tmconfig "github.com/tendermint/tendermint/config"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/pvss"
	"github.com/torusresearch/torus-public/keygen"
	"github.com/torusresearch/torus-public/telemetry"
)

func startAVSSKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) error {
	
	return nil
}