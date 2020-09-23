package sniffer

import (
	"fmt"
	"log"
	"time"

	"github.com/andreaaizza/sniffer/dissector"
	"github.com/andreaaizza/sniffer/logger"
)

// Modbus data for scanning, most frequent first
var ModbusSpeeds = []int{38400, 9600, 19200, 1200, 2400, 4800, 57600, 115200}
var ModbusFrame = []string{"8N1", "8N2"}

type Sniffer struct {
	logger    *logger.Logger
	dissector *dissector.Dissector
}

// Close closes
func (s *Sniffer) Close() {
	// close dissector
	s.dissector.Close()

	// close logger
	s.logger.Close()
}

// NewModbusRTUSniffer creates and starts a sniffer for Modbus RTU
// Process runs on go routine, which can be stopped with Sniffer.Close()
// You need to secure main program does not exit e.g. for{select{}}
func NewModbusRTUSniffer(c *logger.Config) (s *Sniffer, err error) {
	// force flushing logger buffer
	c.FlushAfterSeconds = logger.FlushAfterSecondsModbusRTU

	s = &Sniffer{}

	// creates and starts logger
	logger, err := logger.New(c)
	if err != nil {
		return
	}
	s.logger = logger

	// creates dissector and connects to logger
	s.dissector = dissector.New()
	s.logger.Subscribe(s.dissector.GetConsumer())

	return
}

// GetResults return results, nil on no results
func (s *Sniffer) GetResults() []*dissector.Result {
	return s.dissector.Results.GetResults()
}

// FlushResults clear results queue
func (s *Sniffer) FlushResults() {
	s.dissector.FlushResults()
}

// ProtoBytes extracts results as protobuf Marshalled bytes and flushes
func (s *Sniffer) ProtoBytes() (b []byte, err error) {
	return s.dissector.ProtoBytes()
}

// Scan for Modbus RTU valid serial port configuration
// connect one 485 line to an active line with traffic to run this
func ScanPort(port string, scanForSeconds int) *logger.Config {
	for _, c := range buildConfigs(port) {
		// create sniffer and try finding results for limited time
		err := tryConfig(c, scanForSeconds)
		if err == nil {
			return c
		}
	}
	return nil
}

// buildConfigs builds all possible configs with specific port and speed/frame combinations
func buildConfigs(port string) (cc []*logger.Config) {
	cc = make([]*logger.Config, 0)
	for _, speed := range ModbusSpeeds {
		for _, frame := range ModbusFrame {
			cc = append(cc, &logger.Config{Port: port, Baud: speed, FrameFormat: frame, FlushAfterSeconds: 0})
		}
	}
	return
}

// tryConfig runs a sniffer with specific {config, seconds}, return nil if results are found or specific error
func tryConfig(c *logger.Config, seconds int) (err error) {
	log.Printf("Trying %s %d %s...\n", c.Port, c.Baud, c.FrameFormat)

	s, err := NewModbusRTUSniffer(c)
	defer s.Close()
	if err != nil {
		return fmt.Errorf("Cannot create sniffer")
	}
	for {
		select {
		case <-time.After(time.Duration(seconds) * time.Second):
			results := s.GetResults()

			if len(results) > 0 {
				return
			} else {
				return fmt.Errorf("No valid data recevied")
			}
		}
	}
}
