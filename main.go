package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/tatsushid/go-fastping"
)

// Statistics ...
type Statistics struct {
	Endpoint string    `json:"endpoint"`
	Address  string    `json:"address"`
	Count    int       `json:"count"`
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

	go handleSigTerm(stats)

	if len(os.Args) < 2 {
		stats.Endpoint = "www.google.com"
	} else {
		stats.Endpoint = os.Args[1]
	}

	addy, err := net.ResolveIPAddr("ip4:icmp", stats.Endpoint)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	stats.Address = addy.String()

	for {
		ping(stats, addy)
	}
}

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

		fmt.Printf("IP Addr: %s receive, RTT: %.3f \n", addr.String(), stats.Value)
	}

	stats.Count++
	p.Run()
}

func handleSigTerm(stats *Statistics) {
	// Enable the capture of Ctrl-C, to cleanly close the application
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	_ = <-c

	elapsed := time.Since(stats.Start)

	fmt.Printf("\r")
	fmt.Printf("\r----\nStatistics: elapsed=%s count=%d min=%.3f max=%.3f avg=%.3f\n\n",
		elapsed,
		stats.Count,
		stats.Min,
		stats.Max,
		stats.Avg,
	)

	os.Exit(0)
}
