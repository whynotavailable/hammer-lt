package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"hammer-api/shared"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

func watcher(cli *clientv3.Client) {
	tests := make(map[string]string)

	rch := cli.Watch(context.Background(), "/lt/test/", clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			if ev.Type == clientv3.EventTypePut {
				tests[string(ev.Kv.Key)] = string(ev.Kv.Value)
			} else {
				delete(tests, string(ev.Kv.Key))
			}

			log.Println(tests)
		}
	}
}

// Keep this the same so I can reuse the found KV, it's not dry, but it's safer
// in the case of a redesign of key names
func serverCleaner(cli *clientv3.Client) {
	for {
		resp, err := cli.Get(context.Background(), "/lt/server", clientv3.WithPrefix())
		if err != nil {
			log.Println(err.Error())
		}

		for _, kv := range resp.Kvs {
			var server shared.ServerRegistration
			json.Unmarshal(kv.Value, &server)
			if time.Now().Add(-30 * time.Second).After(server.Timestamp) {
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

	go watcher(cli)
	go serverCleaner(cli)

	http.HandleFunc("/api/test", testRunner(cli))
	http.HandleFunc("/api/servers", listServers(cli))

	log.Println("listening")
	http.ListenAndServe(":8085", nil)
}

func listServers(cli *clientv3.Client) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		servers := getServers(cli)
		response, _ := json.Marshal(servers)
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
		log.Println(test.Targets)
		test.State = "start"

		lease, _ := cli.Lease.Grant(context.Background(), int64(test.Length)+30)
		leaseID := strconv.FormatInt(int64(lease.ID), 16)

		test.Lease = leaseID

		data, _ = json.Marshal(test)
		log.Println(lease.ID)
		lid, _ := strconv.ParseInt(leaseID, 16, 64)
		cli.Put(context.Background(), "/lt/test/"+leaseID, string(data), clientv3.WithLease(clientv3.LeaseID(lid)))
	}
}
