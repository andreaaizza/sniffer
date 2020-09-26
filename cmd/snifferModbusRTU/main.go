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
	port := flag.String("d", "/dev/ttyAPP3", "port")
	baud := flag.Int("b", 9600, "baud")
	frame := flag.String("f", "8N1", "frame config (e.g. \"8N1\")")
	debug := flag.Bool("debug", false, "debug")
	runFor := flag.Int("s", 0, "exit after [seconds]")
	scanOnly := flag.Bool("scan", false, "scan selected port, try finding valid serial config for Modbus RTU")
	scanEachPortSeconds := flag.Int("seconds", ScanSecondsModbusRTUDefault, "try each configuration for [seconds] (default "+
		fmt.Sprintf("%d", ScanSecondsModbusRTUDefault)+")")
	flag.Parse()

	// scan only?
	if *scanOnly {
		fmt.Printf("Scanning port %s...\n", *port)
		c := sniffer.ScanPort(*port, baud, frame, *scanEachPortSeconds, *debug)
		if c != nil {
			fmt.Printf("Found! Received valid data port=%s baud=%d frame=%s\n", c.Port, c.Baud, c.FrameFormat)
			os.Exit(0)
		}
		fmt.Print("No valid config found\n")
		os.Exit(1)
	}

	// sniffer
	conf := logger.Config{Port: *port, Baud: int(*baud), FrameFormat: *frame}
	if *debug {
		fmt.Printf("Starting Modbus RTU sniffer on port: %s, baud: %d, frame format: %s...\n",
			conf.Port, conf.Baud, conf.FrameFormat)
	}
	s, err := sniffer.NewModbusRTUSniffer(&conf)
	if err != nil {
		log.Panic(err)
	}

	// Print results
	go func() {
		for {
			ticker5s := time.NewTicker(5 * time.Second)
			select {
			case <-ticker5s.C:
				results := s.GetResults()

				// if no results
				if len(results) == 0 {
					continue
				}

				// output
				for _, r := range results {
					fmt.Print(r.PrettyString(), "\n")
				}
				// flush
				s.FlushResults()
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
					fmt.Print("Results count: ", len(s.GetResults()), "\n")
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

					os.Exit(0) // TODO wait Closing completes
				}
			}
		}()
	}

	for {
		select {}
	}
}
