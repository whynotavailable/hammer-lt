package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.etcd.io/etcd/clientv3"
	"hammer-api/shared"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func serverCleaner(cli *clientv3.Client) {
	for {
		resp, err := cli.Get(context.Background(), "/lt/server", clientv3.WithPrefix())
		if err != nil {
			log.Println(err.Error())
		}

		for _, kv := range resp.Kvs {
			var server shared.ServerRegistration
			err := json.Unmarshal(kv.Value, &server)

			// If the data doesn't unmarshal, or the timeout's met, delete the registration.
			// If the server comes back it'll just pop back in.
			if err != nil || time.Now().Add(-30*time.Second).After(server.Timestamp) {
				log.Println(fmt.Sprintf("Server %s timed out", server.ID))
				cli.Delete(context.Background(), string(kv.Key))
			}
		}

		time.Sleep(time.Second * 10)
	}
}

func getServers(cli *clientv3.Client) []shared.ServerRegistration {
	resp, err := cli.Get(context.Background(), "/lt/server", clientv3.WithPrefix())
	if err != nil {
		log.Println(err.Error())
	}

	servers := make([]shared.ServerRegistration, 0)

	for _, kv := range resp.Kvs {
		var server shared.ServerRegistration
		json.Unmarshal(kv.Value, &server)
		servers = append(servers, server)
	}

	return servers
}

func main() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	go serverCleaner(cli)

	r := mux.NewRouter()

	r.Use(cors)

	h := hub{
		clients:    make(map[*socketClient]bool),
		register:   make(chan *socketClient),
		unregister: make(chan *socketClient),
		send: make(chan shared.SocketResponse),
	}

	// Actions
	r.HandleFunc("/api/test", testRunner(cli)).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/test", getTest(cli)).Methods("GET", "OPTIONS")

	// Reads (need websockets analogs)
	r.HandleFunc("/api/tests", listTests(cli))
	r.HandleFunc("/api/results", listResults(cli))
	r.HandleFunc("/api/servers", listServers(cli))

	r.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		serveWs(&h, writer, request)
	})

	http.Handle("/", r)

	log.Println("listening")

	go h.Runner()

	go watcher(cli, &h)

	http.ListenAndServe(":8085", nil)
}

func getTest(cli *clientv3.Client) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		testID := request.URL.Query().Get("test")
		resp, err := cli.Get(context.Background(), fmt.Sprintf("/lt/test/%s", testID))

		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte(err.Error()))
		}

		for _, kv := range resp.Kvs {
			var test shared.Test
			json.Unmarshal(kv.Value, &test)

			// clear out the headers
			for i := 0; i < len(test.Targets); i++ {
				test.Targets[i].Headers = nil // These can contain secrets, don't return them
			}

			data, _ := json.Marshal(test)

			writer.Header().Add("Content-Type", "application/json")
			writer.Write(data)

			return
		}

		writer.WriteHeader(http.StatusNotFound)
	}
}

func watcher(cli *clientv3.Client, h *hub) {
	rch := cli.Watch(context.Background(), "/lt/", clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			if ev.Type == clientv3.EventTypePut {
				key := string(ev.Kv.Key)

				if strings.HasPrefix(key, "/lt/test/") {
					log.Println("found test")
					var test shared.Test
					json.Unmarshal(ev.Kv.Value, &test)

					for i := 0; i < len(test.Targets); i++ {
						test.Targets[i].Headers = nil // These can contain secrets, don't return them
					}

					h.send <- shared.SocketResponse{
						Type:     "test",
						Data:     test,
					}
				} else if strings.HasPrefix(key, "/lt/server/") {

				} else if strings.HasPrefix(key, "/lt/results/") {
					var results shared.ResultData
					json.Unmarshal(ev.Kv.Value, &results)

					h.send <- shared.SocketResponse{
						Type:     "results",
						Data:     results,
					}
				}
			}
		}
	}
}

func cors(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Access-Control-Allow-Origin", "http://localhost:4200")
		writer.Header().Add("Access-Control-Allow-Methods", "*")
		writer.Header().Add("Access-Control-Allow-Headers", "*")

		if request.Method != "OPTIONS" {
			handler.ServeHTTP(writer, request)
		}
	})
}

func listResults(cli *clientv3.Client) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		testID := request.URL.Query().Get("test")
		q := fmt.Sprintf("/lt/results/%s/", testID)
		resp, err := cli.Get(context.Background(), q, clientv3.WithPrefix())

		if err != nil {
			log.Println(err.Error())
		}

		results := make([]shared.ResultData, 0)

		for _, kv := range resp.Kvs {
			var testResult shared.ResultData
			json.Unmarshal(kv.Value, &testResult)

			results = append(results, testResult)
		}

		response, _ := json.Marshal(results)
		writer.Header().Add("Content-Type", "application/json")
		writer.Write(response)
	}
}

func listServers(cli *clientv3.Client) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		servers := getServers(cli)
		response, _ := json.Marshal(servers)
		writer.Header().Add("Content-Type", "application/json")
		writer.Write(response)
	}
}

func listTests(cli *clientv3.Client) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		resp, err := cli.Get(context.Background(), "/lt/test", clientv3.WithPrefix())
		if err != nil {
			log.Println(err.Error())
		}

		tests := make([]shared.Test, 0)

		for _, kv := range resp.Kvs {
			var test shared.Test
			json.Unmarshal(kv.Value, &test)

			for i := 0; i < len(test.Targets); i++ {
				test.Targets[i].Headers = nil // These can contain secrets, don't return them
			}

			tests = append(tests, test)
		}

		response, _ := json.Marshal(tests)
		writer.Header().Add("Content-Type", "application/json")
		writer.Write(response)
	}
}

func testRunner(cli *clientv3.Client) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		var test shared.Test
		data, err := ioutil.ReadAll(request.Body)

		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte(err.Error()))
			return
		}

		json.Unmarshal(data, &test)
		test.State = "start"

		lease, _ := cli.Lease.Grant(context.Background(), int64(test.Length)+1800)
		leaseID := strconv.FormatInt(int64(lease.ID), 16)

		test.Lease = leaseID

		data, _ = json.Marshal(test)
		lid, _ := strconv.ParseInt(leaseID, 16, 64)
		cli.Put(context.Background(), "/lt/test/"+leaseID, string(data), clientv3.WithLease(clientv3.LeaseID(lid)))

		response, _ := json.Marshal(shared.TestResponse{ID: leaseID})
		writer.Header().Add("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated) // technically not right, but mostly
		writer.Write(response)
	}
}
