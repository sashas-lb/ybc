package main

import (
	"../../../libs/go/memcache"
	"bytes"
	"flag"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

var (
	connectAddr             = flag.String("connectAddr", ":11211", "Memcached address to test")
	connectionsCount        = flag.Int("connectionsCount", 1, "The number of TCP connection to memcached server")
	goMaxProcs              = flag.Int("goMaxProcs", 4, "The maximum number of simultaneous worker threads in go")
	key                     = flag.String("key", "key", "The key to query in memcache")
	maxPendingRequestsCount = flag.Int("maxPendingRequestsCount", 1024, "Maximum number of pending requests")
	osReadBufferSize        = flag.Int("osReadBufferSize", 224*1024, "The size of read buffer in bytes in OS")
	osWriteBufferSize       = flag.Int("osWriteBufferSize", 224*1024, "The size of write buffer in bytes in OS")
	requestsCount           = flag.Int("requestsCount", 1000*1000, "The number of requests to send to memcache")
	readBufferSize          = flag.Int("readBufferSize", 4096, "The size of read buffer in bytes")
	value                   = flag.String("value", "value", "Value to store in memcache")
	workerMode              = flag.String("workerMode", "GetMiss", "Worker mode. May be 'GetMiss', 'GetHit'")
	workersCount            = flag.Int("workersCount", 512, "The number of workers to send requests to memcache")
	writeBufferSize         = flag.Int("writeBufferSize", 4096, "The size of write buffer in bytes")
)

func workerGetMiss(client *memcache.Client, wg *sync.WaitGroup, ch <-chan int) {
	defer wg.Done()
	item := memcache.Item{
		Key: []byte(*key),
	}
	for _ = range ch {
		if err := client.Get(&item); err != memcache.ErrCacheMiss {
			log.Fatalf("Error in Client.Get(): [%s]", err)
		}
	}
}

func workerGetHit(client *memcache.Client, wg *sync.WaitGroup, ch <-chan int) {
	defer wg.Done()
	item := memcache.Item{
		Key:   []byte(*key),
		Value: []byte(*value),
	}
	if err := client.Set(&item); err != nil {
		log.Fatal("Error in Client.Set(): [%s]", err)
	}
	valueOrig := item.Value
	item.Value = nil
	for _ = range ch {
		if err := client.Get(&item); err != nil {
			log.Fatalf("Error in Client.Get(): [%s]", err)
		}
		if !bytes.Equal(valueOrig, item.Value) {
			log.Fatal("Unexpected value read=[%s]. Expected=[%s]", item.Value, valueOrig)
		}
	}
}

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(*goMaxProcs)
	client := memcache.Client{
		ConnectAddr:             *connectAddr,
		ConnectionsCount:        *connectionsCount,
		MaxPendingRequestsCount: *maxPendingRequestsCount,
		ReadBufferSize:          *readBufferSize,
		WriteBufferSize:         *writeBufferSize,
		OSReadBufferSize:        *osReadBufferSize,
		OSWriteBufferSize:       *osWriteBufferSize,
	}
	client.Start()
	defer client.Stop()

	fmt.Printf("Config:\n")
	fmt.Printf("connectAddr=[%s]\n", *connectAddr)
	fmt.Printf("connectionsCount=[%d]\n", *connectionsCount)
	fmt.Printf("goMaxProcs=[%d]\n", *goMaxProcs)
	fmt.Printf("key=[%s]\n", *key)
	fmt.Printf("maxPendingRequestsCount=[%d]\n", *maxPendingRequestsCount)
	fmt.Printf("osReadBufferSize=[%d]\n", *osReadBufferSize)
	fmt.Printf("osWriteBufferSize=[%d]\n", *osWriteBufferSize)
	fmt.Printf("requestsCount=[%d]\n", *requestsCount)
	fmt.Printf("readBufferSize=[%d]\n", *readBufferSize)
	fmt.Printf("value=[%s]\n", *value)
	fmt.Printf("workerMode=[%s]\n", *workerMode)
	fmt.Printf("workersCount=[%d]\n", *workersCount)
	fmt.Printf("writeBufferSize=[%d]\n", *writeBufferSize)
	fmt.Printf("\n")

	fmt.Printf("Preparing...")
	ch := make(chan int, *requestsCount)
	for i := 0; i < *requestsCount; i++ {
		ch <- 1
	}
	close(ch)
	fmt.Printf("done\n")

	fmt.Printf("starting...")
	startTime := time.Now()
	wg := sync.WaitGroup{}
	defer func() {
		wg.Wait()
		duration := float64(time.Since(startTime)) / float64(time.Second)
		fmt.Printf("done! %.3f seconds, %.0f qps\n", duration, float64(*requestsCount)/duration)
	}()

	worker := workerGetMiss
	if *workerMode == "GetHit" {
		worker = workerGetHit
	}
	for i := 0; i < *workersCount; i++ {
		wg.Add(1)
		go worker(&client, &wg, ch)
	}
}
