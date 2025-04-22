package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	_ "github.com/apache/skywalking-go"
	"github.com/apache/skywalking-go/toolkit/trace"
)

func executeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for /execute")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		trace.CreateLocalSpan("testGoroutineLocalSpan")
		time.Sleep(100 * time.Millisecond)
		trace.StopSpan()
	}()
	wg.Wait()
	log.Println("Goroutine finished, sending response")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func main() {
	http.HandleFunc("/execute", executeHandler)
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	_ = http.ListenAndServe(":8080", nil)
}
