package dkgnode

/* All useful imports */
// import (
// 	"crypto/ecdsa"
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"io/ioutil"
// 	"math/big"
// 	"strings"
// 	"time"

// 	"github.com/Rican7/retry"
// 	"github.com/Rican7/retry/backoff"
// 	"github.com/Rican7/retry/strategy"
// 	tmconfig "github.com/tendermint/tendermint/config"
// 	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
// 	tmnode "github.com/tendermint/tendermint/node"
// 	"github.com/tendermint/tendermint/p2p"
// 	"github.com/tendermint/tendermint/privval"
// 	tmtypes "github.com/tendermint/tendermint/types"
// 	"github.com/torusresearch/torus-public/common"
// 	"github.com/torusresearch/torus-public/keygen"
// 	"github.com/torusresearch/torus-public/logging"
// 	"github.com/torusresearch/torus-public/pvss"
// 	"github.com/torusresearch/torus-public/telemetry"
// )

// import (
// 	"fmt"
// 	"io/ioutil"
// 	"log"

// 	"github.com/gogo/protobuf/proto"
// 	uuid "github.com/google/uuid"
// 	inet "github.com/libp2p/go-libp2p-net"
// 	peer "github.com/libp2p/go-libp2p-peer"
// 	p2p "github.com/torusresearch/torus-public/dkgnode/pb"
// 	"github.com/torusresearch/torus-public/logging"
// )

// // pattern: /protocol-name/request-or-response-message/version
// const KEYGENRequest = "/KEYGEN/KEYGENreq/0.0.1"
// const KEYGENResponse = "/KEYGEN/KEYGENresp/0.0.1"

// // KEYGENProtocol type
// type KEYGENProtocol struct {
// 	localHost *P2PSuite                   // local host
// 	requests  map[string]*p2p. // used to access request data from response handlers
// }

// func startAVSSKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) error {

// 	return nil
// }
