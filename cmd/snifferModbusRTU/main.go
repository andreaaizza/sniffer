package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/andreaaizza/sniffer"
	"github.com/andreaaizza/sniffer/logger"
)

const (
	ScanSecondsModbusRTUDefault = 5
)

func main() {
	// logging flags
	log.SetFlags(log.LstdFlags)

	// flag
	duplex := flag.Bool("duplex", false, "duplex mode. Uses d1 and d2: d1 is TX, d2 is RX")
	port1 := flag.String("d1", "/dev/ttyAPP3", "port1 half-duplex: tx and rx, duplex: tx only")
	port2 := flag.String("d2", "/dev/ttyAPP2", "port2 half-duplex: not used, duplex: rx only")
	baud := flag.Int("b", 9600, "baud")
	frame := flag.String("f", "8N1", "frame config")
	debug := flag.Bool("debug", false, "debug")
	runFor := flag.Int("s", 0, "exits after specified amount of seconds (default 0==infinite)")
	scanOnly := flag.Bool("scan", false, "scans each configuration for scan_seconds. Returns success if at least one request->{response/exception} match is found. In duplex mode, it is not supported to have different baud/frame between tx and rx lines")
	scanEachPortSeconds := flag.Int("scan_seconds", ScanSecondsModbusRTUDefault, "try each configuration for seconds")
	flag.Parse()

	// parse flags
	var conf sniffer.Config
	var s *sniffer.Sniffer
	var err error

	// parse flags
	if *duplex {
		ports := []*logger.Config{
			&logger.Config{Port: *port1, Baud: int(*baud), FrameFormat: *frame, Debug: *debug},
			&logger.Config{Port: *port2, Baud: int(*baud), FrameFormat: *frame, Debug: *debug},
		}
		conf = sniffer.Config{Ports: ports}
		log.Printf("Starting duplex Modbus RTU sniffer on %s %s", ports[0].PrettyString(), ports[1].PrettyString())
	} else {
		ports := []*logger.Config{
			&logger.Config{Port: *port1, Baud: int(*baud), FrameFormat: *frame, Debug: *debug},
		}
		conf = sniffer.Config{Ports: ports}
		log.Printf("Starting half-duplex Modbus RTU sniffer on %s", ports[0].PrettyString())
	}

	// scan only?
	if *scanOnly {
		var baudP *int = nil
		var frameP *string = nil
		if isFlagPassed("b") {
			baudP = baud
		}
		if isFlagPassed("f") {
			frameP = frame
		}

		//fmt.Printf("Scanning port %s...\n", *port)
		c := sniffer.ScanPort(conf, baudP, frameP, *scanEachPortSeconds, *debug)
		if c != nil {
			fmt.Printf("Found! Received valid data with: %s\n", c.PrettyString())
			os.Exit(0)
		}
		fmt.Print("No valid config found\n")
		os.Exit(1)
	}

	// sniffer
	s, err = sniffer.NewModbusRTUSniffer(conf)
	if err != nil {
		log.Panic(err)
	}

	// Print results
	go func() {
		for {
			ticker5s := time.NewTicker(5 * time.Second)
			select {
			case <-ticker5s.C:
				results := s.GetResultsAndFlush()

				// output
				for _, r := range results.GetResults() {
					fmt.Print(r.PrettyString(), "\n")
				}
			}
		}
	}()
	// Count results
	if *debug {
		go func() {
			for {
				ticker1s := time.NewTicker(1 * time.Second)
				select {
				case <-ticker1s.C:
					fmt.Print("Results count: ", s.GetResultsCount(), "\n")
				}
			}
		}()
	}
	// Exit
	if *runFor > 0 {
		go func() {
			for {
				select {
				case <-time.After(time.Duration(*runFor) * time.Second):
					if *debug {
						fmt.Print("Closing sniffer...\n")
					}
					s.Close()

					os.Exit(0)
				}
			}
		}()
	}

	for {
		select {}
	}
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
