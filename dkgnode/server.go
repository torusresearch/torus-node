package dkgnode

/* All useful imports */
import (
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/mholt/certmagic"
	"github.com/patrickmn/go-cache"
	"github.com/rs/cors"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tidwall/gjson"
	"github.com/torusresearch/jsonrpc"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/pvss"
)

func setUpRouter(suite *Suite) http.Handler {
	mr, err := setUpJRPCHandler(suite)
	if err != nil {
		logging.Fatalf("%s", err)
	}

	router := mux.NewRouter().StrictSlash(true)
	notProtected := router.PathPrefix("/").Subrouter()
	notProtected.Use(loggingMiddleware)
	notProtected.HandleFunc("/healthz", GETHealthz)

	// tlsNotProtected := router.PathPrefix("/tls").Subrouter()

	protected := router.PathPrefix("/jrpc").Subrouter()
	protected.Handle("/", mr)
	protected.HandleFunc("/debug", mr.ServeDebug)

	protected.Use(augmentRequestMiddleware)
	protected.Use(loggingMiddleware)
	protected.Use(telemetryMiddleware)

	handler := cors.Default().Handler(router)
	return handler
}

func setUpAndRunHttpServer(suite *Suite) {
	router := setUpRouter(suite)
	addr := fmt.Sprintf(":%s", suite.Config.HttpServerPort)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	if suite.Config.ServeUsingTLS {
		if suite.Config.UseAutoCert {
			if suite.Config.PublicURL == "" {
				logging.Fatal("PublicURL is required when UseAutoCert is true")
			}
			err := certmagic.HTTPS([]string{suite.Config.PublicURL, fmt.Sprintf("www.%s", suite.Config.PublicURL)}, router)
			if err != nil {
				logging.Fatal(err.Error())
			}

		}

		if suite.Config.ServerCert != "" {
			err := server.ListenAndServeTLS(suite.Config.ServerCert,
				suite.Config.ServerKey)
			if err != nil {
				logging.Fatal(err.Error())
			}
		} else {
			logging.Fatal("Certs not supplied, try running with UseAutoCert")
		}

	} else {
		err := server.ListenAndServe()
		if err != nil {
			logging.Fatal(err.Error())
		}

	}

}

func HandleSigncryptedShare(suite *Suite, tx KeyGenShareBFTTx) error {
	p := tx.SigncryptedMessage
	// check if this for us, or for another node
	tmpToPubKeyX, parsed := new(big.Int).SetString(p.ToPubKeyX, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	if suite.EthSuite.NodePublicKey.X.Cmp(tmpToPubKeyX) != 0 {
		logging.Debug("Signcrypted share received but is not addressed to us")
		return nil
	}
	tmpRx, parsed := new(big.Int).SetString(p.RX, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpRy, parsed := new(big.Int).SetString(p.RY, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpSig, parsed := new(big.Int).SetString(p.Signature, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpPubKeyX, parsed := new(big.Int).SetString(p.FromPubKeyX, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpPubKeyY, parsed := new(big.Int).SetString(p.FromPubKeyY, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}

	tmpCiphertext, err := hex.DecodeString(p.Ciphertext)
	if err != nil {
		return jsonrpc.ErrParse()
	}

	signcryption := common.Signcryption{
		Ciphertext: tmpCiphertext,
		R:          common.Point{X: *tmpRx, Y: *tmpRy},
		Signature:  *tmpSig,
	}
	unsigncryptedData, err := pvss.UnsigncryptShare(signcryption, *suite.EthSuite.NodePrivateKey.D, common.Point{*tmpPubKeyX, *tmpPubKeyY})
	if err != nil {
		logging.Errorf("Error unsigncrypting share %s", err)
		return &jsonrpc.Error{Code: 32602, Message: "Invalid params", Data: "error unsigncrypting share " + err.Error()}
	}

	// deserialize share and broadcastId from signcrypted data
	n := len(*unsigncryptedData)
	shareBytes := (*unsigncryptedData)[:n-32]
	broadcastId := (*unsigncryptedData)[n-32:]

	logging.Debugf("KEYGEN: Saved share from %s", p.FromAddress)
	savedLog, found := suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_LOG")
	newShareLog := ShareLog{time.Now().UTC(), 0, int(p.ShareIndex), shareBytes, broadcastId}
	// if not found, we create a new mapping
	if found {
		var tempLog = savedLog.([]ShareLog)
		// newShareLog = ShareLog{time.Now().UTC(), len(tempLog), p.ShareIndex, shareBytes, broadcastId}
		newShareLog.LogNumber = len(tempLog)
		tempLog = append(tempLog, newShareLog)
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", tempLog, cache.NoExpiration)
	} else {
		newLog := make([]ShareLog, 1)
		newLog[0] = newShareLog
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", newLog, cache.NoExpiration)
	}

	savedMapping, found := suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_MAPPING")
	// if not found, we create a new mapping
	if found {
		var tmpMapping = savedMapping.(map[int]ShareLog)
		tmpMapping[int(p.ShareIndex)] = newShareLog
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_MAPPING", tmpMapping, cache.NoExpiration)
	} else {
		newMapping := make(map[int]ShareLog)
		newMapping[int(p.ShareIndex)] = newShareLog
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_MAPPING", newMapping, cache.NoExpiration)
	}
	logging.Debugf("KEYGEN: SAVED SHARE FINISHED for index %d", p.ShareIndex)
	return nil
}

//gets assigned index and returns users public key
func retrieveUserPubKey(suite *Suite, assignedIndex int) (*common.Point, error) {

	resultPubPolys, err := suite.BftSuite.BftRPC.TxSearch("share_index<="+strconv.Itoa(assignedIndex)+" AND "+"share_index>="+strconv.Itoa(assignedIndex), false, 10, 10)
	if err != nil {
		return nil, err
	}
	logging.Debugf("SEARCH RESULT NUMBER %d", resultPubPolys.TotalCount)

	//create users publicKey
	var finalUserPubKey common.Point //initialize empty pubkey to fill
	for i := 0; i < resultPubPolys.TotalCount; i++ {
		var PubPolyTx PubPolyBFTTx
		defaultWrapper := DefaultBFTTxWrapper{&PubPolyTx}
		//get rid of signature on bft
		err = defaultWrapper.DecodeBFTTx(resultPubPolys.Txs[i].Tx[len([]byte("mug00")):])
		if err != nil {
			logging.Debugf("Could not decode pubpolybfttx %s", err)
		}
		pubPolyTx := defaultWrapper.BFTTx.(*PubPolyBFTTx)
		if i == 0 {
			finalUserPubKey = common.Point{pubPolyTx.PubPoly[0].X, pubPolyTx.PubPoly[0].Y}
		} else {
			//get g^z and add
			tempX, tempY := suite.EthSuite.secp.Add(&finalUserPubKey.X, &finalUserPubKey.Y, &pubPolyTx.PubPoly[0].X, &pubPolyTx.PubPoly[0].Y)
			finalUserPubKey = common.Point{*tempX, *tempY}
		}
	}

	return &finalUserPubKey, nil
}

func listenForShares(suite *Suite, count int) {
	logging.Debugf("KEYGEN: listening for shares %d", count)
	query := tmquery.MustParse("keygeneration.sharecollection='1'")
	logging.Debugf("QUERY IS: %s", query)
	// note: we also get back the initial "{}"
	// data comes back in bytes of utf-8 which correspond
	// to a base64 encoding of the original data
	responseCh, err := suite.BftSuite.RegisterQuery(query.String(), count)
	if err != nil {
		logging.Errorf("BFTWS: failure to registerquery %s", query.String())
		return
	}
	for e := range responseCh {
		logging.Debugf("KEYGEN: got a share %s", e)
		// if gjson.GetBytes(e, "query").String() != "keygeneration.sharecollection='1'" {
		// 	continue
		// }
		logging.Debugf("sub got %s", string(e[:]))
		res, err := b64.StdEncoding.DecodeString(gjson.GetBytes(e, "data.value.TxResult.tx").String())
		if err != nil {
			logging.Errorf("error decoding b64 %s", err)
			continue
		}
		// valid messages should start with mug00
		if len(res) < 5 || string(res[:len([]byte("mug00"))]) != "mug00" {
			logging.Debug("Message not prefixed with mug00")
			continue
		}
		keyGenShareBFTTx := DefaultBFTTxWrapper{&KeyGenShareBFTTx{}}
		err = keyGenShareBFTTx.DecodeBFTTx(res[len([]byte("mug00")):])
		if err != nil {
			logging.Debugf("error decoding bfttx %s", err)
			continue
		}
		keyGenShareTx := keyGenShareBFTTx.BFTTx.(*KeyGenShareBFTTx)
		logging.Debugf("KEYGEN: handling signcryption for share %s", keyGenShareTx)
		err = HandleSigncryptedShare(suite, *keyGenShareTx)
		if err != nil {
			logging.Errorf("failed to handle signcrypted share %s", err)
			continue
		}
	}
}
