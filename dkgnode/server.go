package dkgnode

/* All useful imports */
import (
	// b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mholt/certmagic"
	"github.com/patrickmn/go-cache"
	"github.com/rs/cors"
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
	// notProtected := router.PathPrefix("/healthz").Subrouter()
	// notProtected.Use(loggingMiddleware)
	// notProtected.HandleFunc("/", GETHealthz)

	// // tlsNotProtected := router.PathPrefix("/tls").Subrouter()

	// protected := router.PathPrefix("/jrpc").Subrouter()
	// protected.Handle("/", mr)
	// protected.HandleFunc("/debug", mr.ServeDebug)

	// protected.Use(augmentRequestMiddleware)
	// protected.Use(loggingMiddleware)
	// protected.Use(telemetryMiddleware)

	router.Handle("/jrpc", mr)
	router.HandleFunc("/jrpc/debug", mr.ServeDebug)
	router.HandleFunc("/healthz", GETHealthz)

	router.Use(augmentRequestMiddleware)
	router.Use(loggingMiddleware)
	router.Use(telemetryMiddleware)

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
			logging.Info("Starting server with CertMagic...")

			// Force http-01 challenges
			certmagic.Default.DisableTLSALPNChallenge = true

			err := certmagic.HTTPS([]string{suite.Config.PublicURL}, router)
			if err != nil {
				logging.Fatal(err.Error())
			}

		}

		if suite.Config.ServerCert != "" {
			logging.Info("Starting HTTPS server with preconfigured certs...")
			err := server.ListenAndServeTLS(suite.Config.ServerCert,
				suite.Config.ServerKey)
			if err != nil {
				logging.Fatal(err.Error())
			}
		} else {
			logging.Fatal("Certs not supplied, try running with UseAutoCert")
		}

	} else {
		logging.Info("Starting HTTP server...")
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

// retrieveUserPubKey gets assigned index and returns users public key
// DEPREICATED
func retrieveUserPubKey(suite *Suite, keyIndex big.Int) (*common.Point, error) {

	resultPubPolys, err := suite.BftSuite.BftRPC.TxSearch("share_index<="+keyIndex.Text(16)+" AND "+"share_index>="+keyIndex.Text(16), false, 10, 10)
	if err != nil {
		return nil, err
	}
	// resultPubPolys, err := suite.BftSuite.BftRPC.TxSearch("share_index<="+keyIndex.Text(16)+" AND "+"share_index>="+keyIndex.Text(16), false, 10, 10)
	// if err != nil {
	// 	return nil, err
	// }
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
