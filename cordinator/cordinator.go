package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	attackerLock sync.RWMutex
	attackers    = []string{}

	aggregatorLock sync.RWMutex
	aggregator     string

	lock         sync.RWMutex
	systemStatus status
)

func getAttackers() []string {
	attackerLock.RLock()
	defer attackerLock.RUnlock()

	return attackers
}

func addAttacker(att string) {
	attackerLock.Lock()

	for _, attacker := range attackers {
		if attacker == att {
			attackerLock.Unlock()
			return
		}
	}

	attackers = append(attackers, att)
	attackerLock.Unlock()

	lock.Lock()
	systemStatus.Attackers = append(systemStatus.Attackers, serviceStatus{
		Waiting: newTrue(),
		URL:     att,
	})
	lock.Unlock()
}

func getAggregator() string {
	aggregatorLock.RLock()
	defer aggregatorLock.RUnlock()

	return aggregator
}

func setAggregator(aggr string) {
	aggregatorLock.Lock()
	aggregator = aggr
	aggregatorLock.Unlock()

	lock.Lock()
	systemStatus.Aggregator = &serviceStatus{
		Waiting: newTrue(),
		URL:     aggregator,
	}
	lock.Unlock()

}

type status struct {
	Attackers  []serviceStatus `json:"attackers,omitempty"`
	Aggregator *serviceStatus  `json:"aggregators,omitempty"`
}

type serviceStatus struct {
	Healthy   *bool      `json:"healthy,omitempty"`
	LastSeen  *time.Time `json:"lastSeen,omitempty"`
	Waiting   *bool      `json:"waiting,omitempty"`
	Active    *bool      `json:"active,omitempty"`
	Attackers []string   `json:"attackers,omitempty"`

	URL string `json:"url"`
}

func main() {

	go func() {
		for {
			time.Sleep(1 * time.Second)

			client := http.DefaultClient

			for i, attacker := range getAttackers() {

				resp, err := client.Get(fmt.Sprintf("%v/status", attacker))
				if err != nil {
					log.Println(err)
					continue
				}
				byt, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()

				if err != nil {
					log.Println(err)
					continue
				}

				var s serviceStatus
				if err := json.Unmarshal(byt, &s); err != nil {
					log.Println(err)
					continue
				}

				now := time.Now()
				lock.Lock()
				systemStatus.Attackers[i].Waiting = nil
				systemStatus.Attackers[i].LastSeen = &now
				systemStatus.Attackers[i].Active = s.Active
				lock.Unlock()
			}

			if aggregator == "" {
				continue
			}

			resp, err := client.Get(fmt.Sprintf("%v/status", aggregator))

			if err != nil {
				log.Println(err)
				continue
			}
			byt, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				log.Println(err)
				continue
			}

			var s serviceStatus
			if err := json.Unmarshal(byt, &s); err != nil {
				log.Println(err)
				continue
			}

			now := time.Now()
			lock.Lock()
			systemStatus.Aggregator.Waiting = nil
			systemStatus.Aggregator.LastSeen = &now
			lock.Unlock()

			missingAttackers := []string{}
			for _, attacker := range getAttackers() {
				matched := false
				for _, registerdAttacker := range s.Attackers {
					if attacker == registerdAttacker {
						matched = true
						break
					}
				}
				if !matched {
					missingAttackers = append(missingAttackers, attacker)
				}
			}

			if len(missingAttackers) == 0 {
				continue
			}

			log.Println("Few hosts are missing on aggregate. Setting them up")

			var hosts struct {
				Hosts []string `json:"hosts"`
			}

			hosts.Hosts = missingAttackers
			byt, err = json.Marshal(hosts)
			if err != nil {
				panic(err)
			}
			r := bytes.NewBuffer(byt)
			url := fmt.Sprintf("%v/hosts", getAggregator())
			_, err = client.Post(url, "application/json", r)
			if err != nil {
				panic(err)
			}

		}
	}()

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		lock.RLock()
		byt, err := json.Marshal(systemStatus)
		if err != nil {
			panic(err)
		}
		lock.RUnlock()

		w.Write(byt)
	})

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(400)
			return
		}

		client := http.DefaultClient

		for _, attacker := range getAttackers() {
			resp, err := client.Post(fmt.Sprintf("%v/start", attacker), "application/json", bytes.NewBuffer([]byte{}))
			if err != nil {
				log.Println(err)
				continue
			}
			resp.Body.Close()
		}
	})

	http.HandleFunc("/aggregator", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(400)
			return
		}

		var payload struct {
			Aggregator string `json:"aggregator"`
		}

		byt, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}

		if err := json.Unmarshal(byt, &payload); err != nil {
			panic(err)
		}

		setAggregator(payload.Aggregator)
	})

	http.HandleFunc("/attackers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(400)
			return
		}

		var payload struct {
			Attackers []string `json:"attackers"`
		}

		byt, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}

		if err := json.Unmarshal(byt, &payload); err != nil {
			panic(err)
		}

		for _, attacker := range payload.Attackers {
			addAttacker(attacker)
		}

	})

	http.ListenAndServe(":9000", nil)
}

func newTrue() *bool {
	b := true
	return &b
}
