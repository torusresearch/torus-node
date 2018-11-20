package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"

	"github.com/YZhenY/torus/bft"
	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/rs/cors"
)

const bftDatabasePath = "./bft.db"
const port = "7053"

var database bft.Database

type EpochHandler struct {
	bft.EpochHandler
}

type SetEpochHandler struct {
	bft.SetEpochHandler
}

type BroadcastHandler struct {
	bft.BroadcastHandler
}

type RetrieveHandler struct {
	bft.RetrieveHandler
}

func (h EpochHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	epoch, err := database.Epoch()

	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return bft.EpochResult{
		Epoch: epoch,
	}, nil
}

func (h SetEpochHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p bft.SetEpochParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	_, err := database.SetEpoch(p.Epoch)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return bft.SetEpochResult{
		Epoch: p.Epoch,
	}, nil
}

func (h BroadcastHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p bft.BroadcastParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	res, err := database.Broadcast([]byte(p.Data), p.Length)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return bft.BroadcastResult{
		Id: int(id),
	}, nil
}

func (h RetrieveHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p bft.RetrieveParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	data, length, err := database.Retrieve(p.Id)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return bft.RetrieveResult{
		Data:   string(data[:]),
		Length: length,
	}, nil
}

func main() {
	databasePath := flag.String("databasePath", bftDatabasePath, "defaults to "+bftDatabasePath)
	production := flag.Bool("production", false, "defaults to false")
	flag.Parse()

	db, err := sql.Open("sqlite3", *databasePath)
	if err != nil {
		log.Fatal(err)
	}

	database = bft.Database{DB: db}

	// init db if doesn't exist
	statement, _ := db.Prepare("CREATE TABLE IF NOT EXISTS broadcast (id INTEGER PRIMARY KEY AUTOINCREMENT, data BLOB, length INT)")
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
	statement, _ = db.Prepare("CREATE TABLE IF NOT EXISTS state (id INTEGER PRIMARY KEY, label TEXT, number INT, string TEXT)")
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
	statement, _ = db.Prepare("INSERT INTO state (id, label, number, string) SELECT 1, 'epoch', 0, NULL WHERE NOT EXISTS (SELECT * FROM state)")
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}

	mr := jsonrpc.NewMethodRepository()

	// TODO: method params are not case sensitive: works, but is bad
	if err := mr.RegisterMethod("Epoch", EpochHandler{}, bft.EpochParams{}, bft.EpochResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("SetEpoch", SetEpochHandler{}, bft.SetEpochParams{}, bft.SetEpochResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("Broadcast", BroadcastHandler{}, bft.BroadcastParams{}, bft.BroadcastResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("Retrieve", RetrieveHandler{}, bft.RetrieveParams{}, bft.RetrieveResult{}); err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/jrpc", mr)
	mux.HandleFunc("/jrpc/debug", mr.ServeDebug)
	// fmt.Println(port)
	handler := cors.Default().Handler(mux)
	if *production {
		// if err := http.ListenAndServeTLS(":443",
		// 	"/etc/letsencrypt/live/"+suite.Config.HostName+"/fullchain.pem",
		// 	"/etc/letsencrypt/live/"+suite.Config.HostName+"/privkey.pem",
		// 	handler,
		// ); err != nil {
		// 	log.Fatalln(err)
		// }
		log.Fatal("This shouldn't be used in production")
	} else {
		if err := http.ListenAndServe(":"+port, handler); err != nil {
			log.Fatalln(err)
		}
	}
}
