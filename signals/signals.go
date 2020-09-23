package signals

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

var cleaners = make([]func() error, 0)

// AddCleanr add function to be executed at specific signal
func AddCleaner(f func() error) {
	cleaners = append(cleaners, f)
}

// Init signals
func Init() {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			sig := <-sigs
			log.Print("Cought signal: ", sig)
			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				var success bool = true
				for _, f := range cleaners {
					if f() != nil {
						success = false
					}
				}

				if success {
					os.Exit(0)
				} else {
					os.Exit(1)
				}

			}
		}
	}()
}
