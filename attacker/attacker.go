package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

var (
	addr = flag.String("addr", ":8081", "addr to bind attacker to")

	lock    sync.RWMutex
	metrics vegeta.Metrics

	startLock sync.RWMutex
	started   = false
)

func init() {
	flag.Parse()
}

func main() {

	http.HandleFunc("/metrics", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Add("content-type", "application/json")

		lock.RLock()
		defer lock.RUnlock()

		byt, err := json.Marshal(metrics)
		if err != nil {
			log.Println(err)
			resp.WriteHeader(500)
			return
		}

		resp.WriteHeader(200)
		resp.Write(byt)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		startLock.RLock()
		status := started
		startLock.RUnlock()

		p := fmt.Sprintf(`{"active": %v}`, status)
		w.Write([]byte(p))
	})

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != "POST" {
			w.WriteHeader(400)
			return
		}

		startLock.Lock()
		if started {
			w.WriteHeader(400)
			w.Write([]byte(
				`{"error": { "msg": "performance tests already running" } }`))

			startLock.Unlock()
			return
		}
		started = true
		startLock.Unlock()

		go func() {
			for {
				rate := uint64(100) // per second
				duration := 2 * time.Second
				targeter := vegeta.NewStaticTargeter(vegeta.Target{
					Method: "GET",
					URL:    "http://localhost:8080/",
				})
				attacker := vegeta.NewAttacker()

				var m vegeta.Metrics
				for res := range attacker.Attack(targeter, rate, duration) {
					m.Add(res)
				}
				m.Close()

				lock.Lock()
				metrics = m
				lock.Unlock()
			}
		}()

		w.Write([]byte(
			`{"status": { "running": true } }`))
	})

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal(err)
	}

}
