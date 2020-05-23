package main

import (
	"context"
	"encoding/json"
	"go.etcd.io/etcd/clientv3"
	"hammer-api/shared"
	"log"
	"strconv"
)

func registrator(cli *clientv3.Client) {
	reg := shared.Test{
		State:        "start",
		StateReason:  "",
		Length:       15,
		VirtualUsers: 100,
		Servers:      nil,
		Targets: []shared.TestTarget{
			{
				URI: "http://localhost:8080/",
			},
		},
	}

	lease, _ := cli.Lease.Grant(context.Background(), int64(reg.Length) + 30)
	leaseID := strconv.FormatInt(int64(lease.ID), 16)

	reg.Lease = leaseID

	data, _ := json.Marshal(reg)
	log.Println(lease.ID)
	lid, _ := strconv.ParseInt(leaseID, 16, 64)
	cli.Put(context.Background(), "/lt/test/"+leaseID, string(data), clientv3.WithLease(clientv3.LeaseID(lid)))
}

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

func main() {
	log.Println("Starting watcher")

	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	go registrator(cli)

	select {}
}
