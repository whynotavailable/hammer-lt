package main

import (
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}

func handler(writer http.ResponseWriter, request *http.Request) {
	time.Sleep(20 * time.Millisecond)
	writer.Write([]byte("ok"))
}
