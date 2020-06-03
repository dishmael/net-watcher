package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/tatsushid/go-fastping"
)

// Statistics ...
type Statistics struct {
	Hostname string    `json:"source"`
	Endpoint string    `json:"endpoint"`
	Address  string    `json:"address"`
	Count    int64     `json:"count"`
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Avg      float64   `json:"avg"`
	Value    float64   `json:"value"`
	Start    time.Time `json:"-"`
}

func main() {
	stats := &Statistics{
		Count: 0,
		Max:   0,
		Min:   0,
		Avg:   0,
		Value: 0,
		Start: time.Now(),
	}

	// setup a sig handler
	go handleSigTerm(stats)

	// make sure we are running as root
	if os.Geteuid() != 0 {
		fmt.Println("ERROR: net-watcher must be run as root")
		os.Exit(-1)
	}

	// determine hostname and endpoint
	stats.Hostname = getHostname()
	stats.Endpoint = getEndpoint()

	// create new client with default option for server url authenticate by token
	client := influxdb2.NewClient("http://masterpi.localdomain:8086", fmt.Sprintf("%s:%s", "admin", "password"))
	defer client.Close()

	// user blocking write client for writes to desired bucket
	writeAPI := client.WriteApiBlocking("", "homedb")

	addy, err := net.ResolveIPAddr("ip4:icmp", stats.Endpoint)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	stats.Address = addy.String()

	for {
		ping(stats, addy)

		p := influxdb2.NewPoint("heartbeat",
			map[string]string{"source": stats.Hostname, "endpoint": stats.Endpoint, "address": stats.Address, "unit": "response_time"},
			map[string]interface{}{"rtt": stats.Value},
			time.Now())

		writeAPI.WritePoint(context.Background(), p)
	}
}

// ping is the primary worker function
func ping(stats *Statistics, ip *net.IPAddr) {
	p := fastping.NewPinger()
	p.AddIPAddr(ip)

	p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		stats.Value = rtt.Seconds() * 1000

		// Calc Min
		if stats.Value < stats.Min || stats.Min == 0 {
			stats.Min = stats.Value
		}

		// Calc Max
		if stats.Value > stats.Max {
			stats.Max = stats.Value
		}

		// Calc Avg
		stats.Avg = (stats.Avg + stats.Value) / 2

		fmt.Printf("Endpoint: %s, IP Addr: %s, RTT: %.3f \n",
			stats.Endpoint, addr.String(), stats.Value)
	}

	stats.Count++
	p.Run()
}

// handleSigTerm deals with Ctrl+C interrupts
func handleSigTerm(stats *Statistics) {
	// Enable the capture of Ctrl-C, to cleanly close the application
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	_ = <-c

	elapsed := time.Since(stats.Start)

	fmt.Printf("\r")
	fmt.Printf("\r----\nStatistics: elapsed=%s count=%d min=%.3f max=%.3f avg=%.3f\n\n",
		elapsed, stats.Count, stats.Min, stats.Max, stats.Avg,
	)

	os.Exit(0)
}

// getHostname returns the environment variable or local hostname as a default value
func getHostname() string {
	hostname, _ := os.Hostname()

	// open file 'hostname'
	dir, _ := os.Getwd()
	file, err := os.Open(dir + "/hostname")
	if err != nil {
		fmt.Printf("WARN: %s - using '%s'\n", err, hostname)
		return hostname
	}
	defer file.Close()

	// read file 'hostname'
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		hostname = scanner.Text()
		if len(hostname) == 0 {
			hostname, _ := os.Hostname()
			fmt.Printf("WARN: hostname file was empty, using '%s'", hostname)
		}
	}

	return hostname
}

// getEndpoint returns the environment variable or a default value
func getEndpoint() string {
	// use argument
	if len(os.Args) > 1 {
		return os.Args[1]
	}

	// use environment var
	if endpoint, ok := os.LookupEnv("WATCH_ENDPOINT"); ok {
		return endpoint
	}

	// use default
	return "www.google.com"
}
