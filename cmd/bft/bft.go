package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/YZhenY/DKGNode/bft"
	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/rs/cors"
)

const bftDatabasePath = "./bft.db"
const port = "7053"

var database bft.Database

type (
	EchoHandler struct{}
	EchoParams  struct {
		Name string `json:"name"`
	}
	EchoResult struct {
		Message string `json:"message"`
	}
)

func (h EchoHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p EchoParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	epoch, _ := database.Epoch()

	return EchoResult{
		Message: "Hello, " + p.Name + " " + strconv.Itoa(epoch),
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

	mr := jsonrpc.NewMethodRepository()

	// TODO: method params are not case sensitive: works, but is bad
	if err := mr.RegisterMethod("Main.Echo", EchoHandler{}, EchoParams{}, EchoResult{}); err != nil {
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
