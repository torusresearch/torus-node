package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"

	"github.com/YZhenY/DKGNode/bft"
	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/rs/cors"
)

const bftDatabasePath = "./bft.db"
const port = "7053"

var database bft.Database

type (
	EpochHandler struct{}
	EpochParams  struct{}
	EpochResult  struct {
		Epoch int `json:"epoch"`
	}
)

type (
	SetEpochHandler struct{}
	SetEpochParams  struct {
		Epoch int `json:"epoch"`
	}
	SetEpochResult struct {
		Epoch int `json:"epoch"`
	}
)

type (
	BroadcastHandler struct{}
	BroadcastParams  struct {
		Data   string `json:"data"`
		Length int    `json:"length"`
	}
	BroadcastResult struct {
		Id int `json:"id"`
	}
)

type (
	RetrieveHandler struct{}
	RetrieveParams  struct {
		Id int `json:"id"`
	}
	RetrieveResult struct {
		Data   string `json:"data"`
		Length int    `json:"length"`
	}
)

func (h EpochHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	epoch, err := database.Epoch()

	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return EpochResult{
		Epoch: epoch,
	}, nil
}

func (h SetEpochHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p SetEpochParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	_, err := database.SetEpoch(p.Epoch)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return SetEpochResult{
		Epoch: p.Epoch,
	}, nil
}

func (h BroadcastHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p BroadcastParams
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

	return BroadcastResult{
		Id: int(id),
	}, nil
}

func (h RetrieveHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p RetrieveParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	data, length, err := database.Retrieve(p.Id)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Database error", Data: err.Error()}
	}

	return RetrieveResult{
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
	statement, _ := db.Prepare("CREATE TABLE IF NOT EXISTS broadcast (id INTEGER PRIMARY KEY, data BLOB, length INT)")
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
	if err := mr.RegisterMethod("Epoch", EpochHandler{}, EpochParams{}, EpochResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("SetEpoch", SetEpochHandler{}, SetEpochParams{}, SetEpochResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("Broadcast", BroadcastHandler{}, BroadcastParams{}, BroadcastResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("Retrieve", RetrieveHandler{}, RetrieveParams{}, RetrieveResult{}); err != nil {
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
