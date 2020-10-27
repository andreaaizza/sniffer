package sniffer

import (
	"fmt"
	"log"
	sync "sync"
	"time"

	"github.com/andreaaizza/sniffer/dissector"
	"github.com/andreaaizza/sniffer/logger"
	"github.com/andreaaizza/sniffer/util"
	"google.golang.org/protobuf/proto"
)

// ModbusFlushDataOlderThanSeconds APUs older than 5 seconds are to be flushed
const ModbusFlushDataOlderThanSeconds uint = 5

// Modbus data for scanning, most frequent first
var ModbusSpeeds = []int{9600, 19200, 38400, 115200, 57600, 4800, 2400, 1200}

func allModbusFrames() (frames []string) {
	frames = make([]string, 0)
	b := []string{"8", "7"}
	p := []string{"N", "E", "O"}
	s := []string{"1", "2", "15"}
	for _, bb := range b {
		for _, pp := range p {
			for _, ss := range s {
				frames = append(frames, bb+pp+ss)
			}
		}
	}
	return
}

type Sniffer struct {
	dissector []*dissector.Dissector

	Results Results
	resMux  sync.Mutex

	stop chan struct{}
}

// Close closes
func (s *Sniffer) Close() {
	// close dissector
	for _, d := range s.dissector {
		d.Close()
	}
}

type Config struct {
	Ports []*logger.Config
}

func (c *Config) PrettyString() (s string) {
	for _, p := range c.Ports {
		s += p.PrettyString() + " "
	}
	return
}

// NewModbusRTUSniffer creates and starts a sniffer for Modbus RTU
// Process runs on go routine, which can be stopped with Sniffer.Close()
// You need to secure main program does not exit e.g. for{select{}}
// if 1 port is provided, then it sniffs half-duples
// if 2 ports are provided, then is sniffs duplex (Requests on port[0] (tx), Responses/Exception on port[1] (rx)
func NewModbusRTUSniffer(conf Config) (s *Sniffer, err error) {

	if len(conf.Ports) == 0 || len(conf.Ports) > 2 {
		log.Panic("Sniffer should have either 1 or 2 ports as input")
	}
	isDuplex := len(conf.Ports) == 2

	// create sniffer
	s = &Sniffer{
		dissector: make([]*dissector.Dissector, 0),
	}

	// set logger flushing time
	for _, p := range conf.Ports {
		p.FlushAfterSeconds = logger.LoggerFlushAfterSecondsModbusRTU
	}

	// creates dissectors
	if isDuplex {
		var txDiss *dissector.Dissector
		var rxDiss *dissector.Dissector

		// port[0] is tx
		txDiss, err = dissector.New(conf.Ports[0], dissector.FilterOnlyModbusRequest{})
		if err != nil {
			return
		}
		// port[1] is rx
		rxDiss, err = dissector.New(conf.Ports[1], dissector.FilterOnlyModbusResponseOrException{})
		if err != nil {
			return
		}
		s.dissector = append(s.dissector, txDiss)
		s.dissector = append(s.dissector, rxDiss)
	} else {
		var txrx *dissector.Dissector

		// port[0] is both tx and rx
		txrx, err = dissector.New(conf.Ports[0], dissector.FilterAnyModbus{})
		if err != nil {
			return
		}
		s.dissector = append(s.dissector, txrx)
	}

	// results buffers
	rx := []dissector.Result{}
	tx := []dissector.Result{}

	if isDuplex {
		// DUPLEX
		go func() {
			for {
				select {
				case <-s.stop:
					return

				// only TX (Requests)
				case r := <-s.dissector[0].Producer:
					tx = append(tx, r)

				// only RX (Responses/Exceptions)
				case r := <-s.dissector[1].Producer:
					// fill queue
					rx = append(rx, r)

					s.findRxTxMatch(&rx, &tx)
				}
			}
		}()
	} else {
		// HALF DUPLEX
		go func() {
			for {
				select {
				case <-s.stop:
					return

				// both Requests and Responses/Exceptions
				case r := <-s.dissector[0].Producer:
					adu := r.GetAdu()
					if adu.IsRequest() {
						tx = append(tx, r)
						break
					} else if adu.IsException() || adu.IsResponse() {
						rx = append(rx, r)

						s.findRxTxMatch(&rx, &tx)

						break
					}
					log.Printf("Unhandled adu received: %v", adu)
				}
			}
		}()
	}

	return
}

func (s *Sniffer) findOneMatch(rx *[]dissector.Result, tx *[]dissector.Result) (found bool) {
	// for each REQ in time ascending order
	for ti := range *tx {
		// find the nearest (in time) future REX/EXC
		for ri := range *rx {
			aduTx := (*tx)[ti].GetAdu()
			aduRx := (*rx)[ri].GetAdu()
			aduTxTime := util.TimeBuilder(aduTx.GetTime())
			aduRxTime := util.TimeBuilder(aduRx.GetTime())

			if aduTxTime.Before(aduRxTime) &&
				aduTx.GetAddress() == aduRx.GetAddress() &&
				((aduRx.IsResponse() &&
					aduRx.GetPduResponse().GetFunctionCode() == aduTx.GetPduRequest().GetFunctionCode()) ||
					(aduRx.IsException() &&
						aduRx.GetPduResponseException().GetFunctionExceptionCode()&0x7F == aduTx.GetPduRequest().GetFunctionCode())) {
				// match found
				res := Result{Request: &dissector.Result{Adu: aduTx}, Response: &dissector.Result{Adu: aduRx}}
				s.resMux.Lock()
				s.Results.Results = append(s.Results.Results, &res)
				s.resMux.Unlock()

				//log.Print("FOUND: ", res) //LOG

				// remove from tx and rx
				*tx = append((*tx)[:ti], (*tx)[ti+1:]...)
				*rx = append((*rx)[:ri], (*rx)[ri+1:]...)
				return true
			}
		}
		// req (tx) has no matching res (rx)
	}
	return false
}

func (s *Sniffer) findRxTxMatch(rx *[]dissector.Result, tx *[]dissector.Result) {
	// flush old data first
	now := time.Now()
	flushOldData(rx, now)
	flushOldData(tx, now)

	// for each REQ find matching RES/EXC
	for {
		if s.findOneMatch(rx, tx) == false {
			break
		}
	}
}

// GetResults return results, and flushes
func (s *Sniffer) GetResultsAndFlush() (res Results) {
	s.resMux.Lock()
	res = s.Results
	s.flushResults()
	s.resMux.Unlock()
	return
}

func (s *Sniffer) GetResultsCount() int {
	return len(s.Results.Results)
}

// FlushResults clear results queue
func (s *Sniffer) flushResults() {
	s.Results.Reset()
}

// ProtoBytes extracts results as protobuf Marshalled bytes
func (s *Sniffer) ProtoBytesAndFlush() (b []byte, err error) {
	s.resMux.Lock()
	defer s.resMux.Unlock()

	b, err = proto.Marshal(&s.Results)
	if err != nil {
		return
	}
	s.flushResults()
	return
}

func (r *Result) PrettyString() string {
	return fmt.Sprint(r.Request.PrettyString(), " -> ", r.Response.PrettyString())
}

// Scan for Modbus RTU valid serial port configuration
// connect one 485 line to an active line with traffic to run this
func ScanPort(conf Config, speed *int, frame *string, scanForSeconds int, debug bool) *Config {
	port := []string{}
	for _, c := range conf.Ports {
		port = append(port, c.Port)
	}
	configs := buildConfigs(port, speed, frame, debug)

	for _, c := range configs {
		// create sniffer and try finding results for limited time
		err := tryConfig(c, scanForSeconds)
		if err == nil {
			return &c
		} else {
			if debug {
				log.Print(err)
			}
		}

	}
	return nil
}

// buildConfigs builds all possible configs with specific port and speed/frame combinations
func buildConfigs(port []string, thisSpeed *int, thisFrame *string, debug bool) (confs []Config) {
	confs = make([]Config, 0)
	for _, speed := range ModbusSpeeds {
		for _, frame := range allModbusFrames() {
			if thisSpeed != nil && *thisSpeed != speed {
				continue
			}
			if thisFrame != nil && *thisFrame != frame {
				continue
			}
			conf := Config{}
			for _, p := range port {
				conf.Ports = append(conf.Ports, &logger.Config{Port: p, Baud: speed, FrameFormat: frame, FlushAfterSeconds: 0, Debug: debug})
			}
			confs = append(confs, conf)
		}
	}
	return
}

// tryConfig runs a sniffer with specific {config, seconds}, return nil if results are found or specific error
func tryConfig(c Config, seconds int) (err error) {

	log.Printf("Trying %s", c.PrettyString())

	s, err := NewModbusRTUSniffer(c)
	if err != nil {
		return fmt.Errorf("Cannot create sniffer")
	}
	defer s.Close()

	for {
		select {
		case <-time.After(time.Duration(seconds) * time.Second):
			results := s.GetResultsAndFlush()

			if len(results.GetResults()) > 0 {
				return
			} else {
				return fmt.Errorf("No valid data recevied")
			}
		}
	}
}

func flushOldData(r *[]dissector.Result, now time.Time) {
	count := 0
	for i := len(*r) - 1; i >= 0; i-- {
		if now.After((*r)[0].GetAdu().GetTimeTime().Add(time.Duration(ModbusFlushDataOlderThanSeconds) * time.Second)) {
			*r = append((*r)[:i], (*r)[i+1:]...)
			count++
		}
	}
	log.Printf("Flushed %d ADUs from buffer", count)
}
