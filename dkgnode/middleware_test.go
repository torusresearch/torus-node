package dkgnode

import (
	"bytes"
	ctx "context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/torusresearch/torus-public/logging"
)

func checkContextHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		method := context.Get(r, "JRPCMethod")
		val, ok := method.(string)
		if !ok {
			t.Fatal("method not found")
		}

		if val != "TestMethod" {
			t.Fatalf("invalid method expected: %s, got %s", "TestMethod", val)
		}
		w.WriteHeader(200)
	}
}

func TestBasicMiddlewareSetup(t *testing.T) {
	targetPort := "8088"
	basicRouter := mux.NewRouter().StrictSlash(true)

	basicRouter.Use(augmentRequestMiddleware)
	basicRouter.Use(loggingMiddleware)
	basicRouter.Use(telemetryMiddleware)

	ch := checkContextHandler(t)
	basicRouter.HandleFunc("/test", ch)
	addr := fmt.Sprintf(":%s", targetPort)
	handler := cors.Default().Handler(basicRouter)

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			t.Fatal(err)
		}
	}()

	idleConnsClosed := make(chan struct{})
	go func() {
		<-idleConnsClosed
		logging.Info("Shutting down server")

		// We received an interrupt signal, shut down.
		err := server.Shutdown(ctx.Background())
		if err != nil {
			// Error from closing listeners, or context timeout:
			logging.Errorf("Error during shutdown: %s", err.Error())
		}

	}()

	client := &http.Client{}
	postURL := fmt.Sprintf("http://localhost:%s/test", targetPort)
	req := jRPCRequeust{
		Method: "TestMethod",
	}
	reqBytes, err := json.Marshal(&req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Post(postURL, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("servered returned status %d but expected %d", resp.StatusCode, 200)
	}

	idleConnsClosed <- struct{}{}
}
