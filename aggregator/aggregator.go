package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

var (
	addr = flag.String("addr", ":8083", "addr to bind attacker to")

	hostsMutex sync.RWMutex
	hosts      = []string{}

	lock         sync.RWMutex
	metricsParts []vegeta.Metrics
)

func getHosts() []string {
	hostsMutex.RLock()
	defer hostsMutex.RUnlock()

	return hosts
}

func addHost(host string) {
	hostsMutex.Lock()

	for _, h := range hosts {
		if h == host {
			hostsMutex.Unlock()
			return
		}
	}

	hosts = append(hosts, host)
	hostsMutex.Unlock()
}

func main() {
	go func() {
		client := http.DefaultClient
		for {

			time.Sleep(1 * time.Second)
			hosts := getHosts()
			parts := make([]vegeta.Metrics, len(hosts))
			for i, host := range hosts {
				resp, err := client.Get(fmt.Sprintf("%v/metrics", host))

				if err != nil {
					log.Println(err)
					continue
				}
				defer resp.Body.Close()

				byt, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					// should have back-off and fail if X % of resp.Body reading fails
					continue
				}
				var metrics vegeta.Metrics
				if err := json.Unmarshal(byt, &metrics); err != nil {
					log.Println(err)
					// should have back-off and fail if X % if payload cannot be parsed
					continue
				}
				parts[i] = metrics
			}
			lock.Lock()
			metricsParts = parts
			lock.Unlock()
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers",
				"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		}
		// Stop here if its Preflighted OPTIONS request
		if r.Method == "OPTIONS" {
			return
		}

		lock.RLock()
		defer lock.RUnlock()

		byt, err := json.Marshal(metricsParts)
		if err != nil {
			// not sure why it would ever panic here
			log.Fatal(err)
		}

		w.Header().Set("content-type", "application/json")
		w.Write(byt)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		var status struct {
			Active    bool     `json:"active,omitempty"`
			Attackers []string `json:"attackers,omitempty"`
		}

		status.Active = true
		status.Attackers = getHosts()

		byt, err := json.Marshal(status)
		if err != nil {
			panic(err)
		}

		w.Write(byt)
	})

	http.HandleFunc("/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			hosts := getHosts()
			byt, err := json.Marshal(hosts)
			if err != nil {
				panic(err)
			}
			w.Write(byt)
			return
		}

		if r.Method == "POST" {

			byt, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close()

			if err != nil {
				panic(err)
			}

			var payload struct {
				Hosts []string `json:"hosts"`
			}

			err = json.Unmarshal(byt, &payload)
			if err != nil {
				panic(err)
			}

			for _, host := range payload.Hosts {
				addHost(host)
			}
		}
	})

	log.Printf("Aggreagor is running at %v\n", *addr)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}
