package logger

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/andreaaizza/sniffer/util"
	"github.com/tarm/serial"
)

const (
	// bufSize read buffer size
	bufSize = 256
)

const (
	// FlushAfterSecondsModbusRTU flush data from logger, if data older than this [seconds]
	FlushAfterSecondsModbusRTU = 10
)

type Config struct {
	Port        string
	Baud        int
	FrameFormat string

	FlushAfterSeconds int

	Debug bool
}

type Logger struct {
	LoggerBuffer
	consumers []chan DataUnit

	serialPort interface{}
	stop       chan struct{}

	config Config
}

// New builds new logger with specified Config
func New(c *Config) (l *Logger, err error) {
	consumers := make([]chan DataUnit, 0)
	l = &Logger{
		consumers: consumers,
		config:    *c,

		stop: make(chan struct{}, 0),
	}
	err = l.initLoggerBuffer()
	return
}

// getSerialConfig returns config built from specific logger config string
func (l *Logger) getSerialConfig() (c *serial.Config, err error) {
	size, err := strconv.Atoi(l.config.FrameFormat[:1])
	if err != nil {
		return
	}
	var par serial.Parity
	switch l.config.FrameFormat[1:2] {
	case "N":
		par = serial.ParityNone
	case "E":
		par = serial.ParityEven
	case "O":
		par = serial.ParityOdd
	case "M": // TODO
		par = serial.ParityMark
	case "S": // TODO
		par = serial.ParitySpace
	default:
		err = fmt.Errorf("invalid frame string")
		return
	}
	var stp serial.StopBits
	switch l.config.FrameFormat[2:3] {
	case "1":
		stp = serial.Stop1
	case "2":
		stp = serial.Stop2
	case "15":
		stp = serial.Stop1Half
	default:
		err = fmt.Errorf("invalid stop bits")
		return
	}

	c = &serial.Config{Name: l.config.Port, Baud: l.config.Baud,
		Size: byte(size), Parity: par, StopBits: stp}
	return
}

// Unsubscrbe deletes all subscriptions
func (l *Logger) Unsubscribe() {
	l.consumers = l.consumers[:0]
}

// Subscribe sends each new DataUnit to specified channel
func (l *Logger) Subscribe(c chan DataUnit) {
	l.consumers = append(l.consumers, c)
}

// Close closes
func (l *Logger) Close() {
	// unsubscribe
	l.Unsubscribe()

	// terminate go routing
	close(l.stop)

	// close port
	port, ok := l.serialPort.(serial.Port)
	if ok {
		port.Flush()
		port.Close()
	} else {
		log.Print("serialPort type not handled")
	}

	// Reset
	l.Reset()
}

// Size in bytes
func (lb LoggerBuffer) Size() int {
	return len(lb.DataUnit)
}

func (lb LoggerBuffer) PrettyString() (s string) {
	for _, du := range lb.GetDataUnit() {
		s += du.PrettyString()
	}
	return s
}

func (du DataUnit) PrettyString() string {
	return fmt.Sprintf("[%v]%02X[%03d]", util.TimeBuilder(*du.GetTime()).UnixNano(), du.GetData(), len(du.GetData()))
}

// initLoggerBuffer builds new and starts collecting data
func (l *Logger) initLoggerBuffer() (err error) {
	// build
	l.LoggerBuffer = LoggerBuffer{}

	// get serial data
	lConfig, err := l.getSerialConfig()
	if err != nil {
		return
	}

	port, err := serial.OpenPort(lConfig)
	if err != nil {
		return
	}
	l.serialPort = *port

	go func() {
		for {
			select {
			case <-l.stop:
				return
			default:
				// get data
				buf := make([]byte, bufSize)

				n, err := port.Read(buf)
				time := time.Now().UTC()
				if err != nil {
					log.Print(err)
					continue
				}
				//log.Printf("NEW DATA [%03d]: %02X %03d", n, buf[:n], buf[:n]) // LOG

				// push to LoggerBuffer
				t := util.TimestampBuilder(time)
				du := DataUnit{
					Data: buf[:n],
					Time: &t,
				}

				l.DataUnit = append(l.DataUnit, &du)

				// feed consumers
				for _, c := range l.consumers {
					c <- du

					if l.config.Debug {
						log.Print("Data received: ", du.PrettyString())
					}
				}

				// Flush every time new data is received
				l.flush()
			}
		}
	}()

	return
}

// flush flushes
func (l *Logger) flush() {
	to := l.config.FlushAfterSeconds
	if to == 0 {
		return
	}
	for i := len(l.DataUnit) - 1; i > 0; i-- {
		t := time.Now().UTC()
		td := util.TimeBuilder(*l.DataUnit[i].GetTime())
		if t.After(td.Add(time.Duration(to) * time.Second)) {
			//log.Print("Flushing logging buffer: ", l.DataUnit[i]) //LOG
			l.DataUnit = l.DataUnit[:i]
		}
	}
}
