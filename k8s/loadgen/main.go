package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

type endpoint struct {
	path   string
	weight int // relative traffic weight
}

var endpoints = []endpoint{
	{"/healthz",   5},
	{"/orders",    40},
	{"/payments",  25},
	{"/inventory", 25},
	{"/slow",      5},
}

func weightedRandom(eps []endpoint) string {
	total := 0
	for _, e := range eps {
		total += e.weight
	}
	r := rand.Intn(total)
	for _, e := range eps {
		r -= e.weight
		if r < 0 {
			return e.path
		}
	}
	return eps[0].path
}

func sendRequest(baseURL string, client *http.Client) {
	path := weightedRandom(endpoints)
	url := baseURL + path

	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("error requesting %s: %v", path, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("path=%s status=%d latency=%dms", path, resp.StatusCode, time.Since(start).Milliseconds())
}

func runWorker(baseURL string, rps int, stop <-chan struct{}) {
	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			go sendRequest(baseURL, client)
		}
	}
}

func main() {
	baseURL := os.Getenv("TARGET_URL")
	if baseURL == "" {
		baseURL = "http://observatory-app"
	}

	log.Printf("Load generator starting — target: %s", baseURL)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Normal load: 10 RPS baseline
	wg.Add(1)
	go func() {
		defer wg.Done()
		runWorker(baseURL, 10, stop)
	}()

	// Traffic spike every 2 minutes — adds 40 RPS for 30 seconds
	go func() {
		for {
			time.Sleep(2 * time.Minute)
			fmt.Println("--- traffic spike starting ---")
			spikeDone := make(chan struct{})
			for i := 0; i < 4; i++ {
				go runWorker(baseURL, 10, spikeDone)
			}
			time.Sleep(30 * time.Second)
			close(spikeDone)
			fmt.Println("--- traffic spike ended ---")
		}
	}()

	wg.Wait()
}
