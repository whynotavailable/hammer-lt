package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"go.etcd.io/etcd/clientv3"
	"hammer-api/shared"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"
)

func registrator(cli *clientv3.Client, serverID string) {
	for {
		reg := shared.ServerRegistration{
			ID:        serverID,
			Timestamp: time.Now(),
		}
		data, _ := json.Marshal(reg)
		cli.Put(context.Background(), "/lt/server/"+serverID, string(data))
		time.Sleep(15 * time.Second)
	}
}

func watcher(cli *clientv3.Client, serverID string) {
	tests := make(map[string]string)

	rch := cli.Watch(context.Background(), "/lt/test/", clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			if ev.Type == clientv3.EventTypePut {
				tests[string(ev.Kv.Key)] = string(ev.Kv.Value)
				var test shared.Test
				json.Unmarshal(ev.Kv.Value, &test)
				for _, server := range test.Servers {
					if server == serverID {
						go runTest(test, cli, serverID)
						break
					}
				}
			} else {
				delete(tests, string(ev.Kv.Key))
			}
		}
	}
}

func runTest(test shared.Test, cli *clientv3.Client, serverID string) {
	log.Println("starting test")

	collector := make(chan shared.TestResult, 50)

	cancellation := make(chan bool)

	var i int16
	for i = 0; i < test.VirtualUsers; i++ {
		go actuallyRunTest(test, collector, cancellation)
	}

	agg := time.After(5 * time.Second)
	timeout := time.After(time.Duration(test.Length) * time.Second)
	results := make(map[string][]int)

	for {
		select {
		case <-timeout:
			log.Println("got timeout")
			for i = 0; i < test.VirtualUsers; i++ {
				cancellation <- true
			}
			aggregate(test, cli, results, serverID)
			log.Println("no more results")
			return
		case <-agg:
			aggregate(test, cli, results, serverID)
			agg = time.After(5 * time.Second)
		case r := <-collector:
			if m := results[r.Target]; m == nil {
				results[r.Target] = make([]int, 0)
			}

			results[r.Target] = append(results[r.Target], r.ResponseTime)
		}
	}
}

func aggregate(test shared.Test, cli *clientv3.Client, results map[string][]int, serverID string) {
	response := shared.ResultData{
		ServerID: serverID,
	}

	log.Println("building agg")

	for targetId, list := range results {
		resultsLen := len(list)
		if resultsLen == 0 {
			return
		}

		sum := 0

		for _, num := range list {
			sum = sum + num
		}

		sort.Sort(sort.IntSlice(list))

		p50 := int(50/float64(100)*float64(len(list))) + 1
		p90 := int(90/float64(100)*float64(len(list))) + 1
		p99 := int(99/float64(100)*float64(len(list))) + 1

		if p50 >= resultsLen {
			p50 = resultsLen - 1
		}

		if p90 >= resultsLen {
			p90 = resultsLen - 1
		}

		if p99 >= resultsLen {
			p99 = resultsLen - 1
		}

		agg := shared.AggregateTestResult{
			Target:            targetId,
			Requests:          resultsLen,
			StatusCodes:       nil,
			P50:               list[p50],
			P90:               list[p90],
			P99:               list[p99],
			RequestsPerSecond: float64(resultsLen) / (float64(sum) / 1000),
		}

		response.Results = append(response.Results, agg)
	}

	data, _ := json.Marshal(response)
	lid, _ := strconv.ParseInt(test.Lease, 16, 64)
	_, err := cli.Put(context.Background(), "/lt/results/"+test.Lease+"/"+serverID, string(data), clientv3.WithLease(clientv3.LeaseID(lid)))

	if err != nil {
		log.Println(err.Error())
	}
}

func actuallyRunTest(test shared.Test, collector chan shared.TestResult, cancellation chan bool) {
	for {
		select {
		case <-cancellation:
			return
		default:
			for _, target := range test.Targets {
				start := time.Now()
				resp, err := http.DefaultClient.Get(target.URI)
				diff := time.Since(start)
				if err != nil {
					log.Println(err.Error())
				}

				results := shared.TestResult{
					StatusCode:   int16(resp.StatusCode),
					ResponseTime: int(diff.Milliseconds()),
					Target:       target.Method + " " + target.URI,
				}

				collector <- results

				time.Sleep(1 * time.Second)
			}
		}
	}
}

func main() {
	uuid, _ := uuid.NewRandom()
	serverID := uuid.URN()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	log.Println("Starting watcher")
	go watcher(cli, serverID)

	log.Println("Starting registration")
	go registrator(cli, serverID)

	select {}
}
