package dissector

import (
	"github.com/andreaaizza/sniffer/logger"
	"github.com/andreaaizza/sniffer/util"

	"fmt"
	"log"
	"time"
)

const (
	// DBMaxSizeWithoutNotify if more DataUnits than this in the buffer, then notity
	DBMaxSizeWithoutNotify = 4096

	// DissectorFlushAfterSecondsModbusRTU flush data if older than this [seconds]
	DissectorFlushAfterSecondsModbusRTU = 5
)

type ResultFilter interface {
	validate(r *Result) bool
}

type FilterOnlyModbusRequest struct{}

func (f FilterOnlyModbusRequest) validate(r *Result) bool {
	return r.GetAdu().IsRequest()
}

type FilterOnlyModbusResponseOrException struct{}

func (f FilterOnlyModbusResponseOrException) validate(r *Result) bool {
	return r.GetAdu().IsResponse() || r.GetAdu().IsException()
}

type FilterAnyModbus struct{}

func (f FilterAnyModbus) validate(r *Result) bool {
	return true
}

type Dissector struct {
	DissectorBuffer
	logger   *logger.Logger
	Consumer chan logger.DataUnit

	Producer chan Result

	stop chan struct{}

	flushDissectorAfterSeconds int

	filter ResultFilter
}

// New builds new dissector and starts waiting for data.
// DataUnits should be sent to GetConsumer() channel. Results can be fetched with GetResults(). Should be closed with Close() at the end.
func New(c *logger.Config, filter ResultFilter) (d *Dissector, err error) {
	// creates and starts logger
	l, err := logger.New(c)
	if err != nil {
		return
	}

	d = &Dissector{
		DissectorBuffer: DissectorBuffer{},
		Consumer:        make(chan logger.DataUnit),
		logger:          l,

		Producer: make(chan Result),

		stop: make(chan struct{}, 0),

		flushDissectorAfterSeconds: DissectorFlushAfterSecondsModbusRTU,
	}

	// connect to logger
	d.logger.Subscribe(d.GetConsumer())

	// assign filter
	d.filter = filter

	go func() {
		for {
			select {
			case <-d.stop:
				return
			case du := <-d.Consumer:
				d.loadDataUnit(&du)

				// dissect after each packet recevied
				d.dissect()
			}
		}
	}()

	return
}

// Close closes
func (d *Dissector) Close() {
	// close dissector
	close(d.stop)

	// close logger
	d.logger.Close()
}

// GetConsumer return the channel to send DataUnits to
func (d *Dissector) GetConsumer() chan logger.DataUnit {
	return d.Consumer
}

/*
// FlushResults clear results queue
func (d *Dissector) FlushResults() {
	d.resultsMux.Lock()
	d.Results.Results = d.Results.Results[:0]
	d.resultsMux.Unlock()
}
*/

// loadDataUnit pushes DataUnit to dissector
func (d *Dissector) loadDataUnit(du *logger.DataUnit) {
	for _, dByte := range du.Data {
		d.TimedBytes = append(d.TimedBytes, &TimedByte{Time: du.Time, Byte: uint32(dByte)})
	}
}

// flushOldData flushes data if too old
func (d *Dissector) flushOldData() {
	t := time.Now().UTC()

	// TimedBytes
	for i := len(d.TimedBytes) - 1; i > 0; i-- {
		td := util.TimeBuilder(d.TimedBytes[i].GetTime())
		if t.After(td.Add(time.Duration(d.flushDissectorAfterSeconds) * time.Second)) {
			d.removeTimedBytes(i, 1)
		}
	}
}

// dissect repeats single dissectRound
func (d *Dissector) dissect() {
	// needs to cycle because data is removed from buffer and buffer changes indexing order
	for {
		foundMatch := d.dissectRound()
		if !foundMatch {
			break
		}

		// flush old data from DissectorBuffer and Results
		d.flushOldData()

		if d.Size() > DBMaxSizeWithoutNotify {
			log.Printf("DissectorBuffer too big. Size=%d. Content: %s", d.Size(), d.PrettyString())
		}
	}
}

// tries finding req->resp couple, returns true if success
func (d *Dissector) dissectRound() bool {
	// cycle thru each byte and search for a valid Request
	for reqIndex, _ := range d.TimedBytes {
		// try building ADU
		if adu, err := NewADU(&d.DissectorBuffer, reqIndex); err == nil {
			res := &Result{Adu: adu}
			// validate
			if d.filter.validate(res) {
				// push to output
				result := Result{Adu: res.GetAdu()}
				d.Producer <- result

				// remove relevant data from input
				d.removeTimedBytes(reqIndex, res.GetAdu().Size())

				return true
			}
		}
	}
	return false
}

// removeTimedBytes removes `size` data from buffer at `start` position
func (d *Dissector) removeTimedBytes(start int, size int) {
	d.TimedBytes = append(d.TimedBytes[:start], d.TimedBytes[start+size:]...)
}

/*
LB errors: [1600174478]020300090001543B[1600174478]02830230F1[1600174478]02030000000AC5FE[1600174478]02830230F1[1600174480]020300090001543B[1600174480]02830230F1[1600174480]02030000000AC5FE[1600174480]02830230F1[1600174482]020300090001543B[1600174482]02830230F1[1600174482]02030000000AC5FE[1600174482]02830230F1[1600174484]020300090001543B[1600174484]02830230F1[1600174484]02030000000AC5FE[1600174484]02830230F1
*/

// bytes returns `size` []bytes from `start`
func (db *DissectorBuffer) bytes(start int, size int) (b []byte, err error) {
	if start+size > db.Size() {
		err = fmt.Errorf("out of bounds")
		return
	}
	for i := 0; i < size; i++ {
		b = append(b, byte(db.TimedBytes[start+i].GetByte()))
	}
	return
}

// findModbusReply find suitable reply to PDURequest at index mdReqADUindex. Returns PDUResponse, position (success), PDUResponseException, 0 (err!=nil)
func (db *DissectorBuffer) findModbusReply(mdReqADUindex int) (r *ADU, i int, err error) {
	var reqADU *ADU
	if reqADU, err = NewADU(db, mdReqADUindex); err != nil || !reqADU.IsRequest() {
		err = fmt.Errorf("should find an ADU with valid PDURequest at index %d", i)
		log.Print(err)
		return

	}
	for i = mdReqADUindex + reqADU.Size(); i < db.Size()-ADUMinSize; i++ {
		if r, err = NewADU(db, i); err != nil {
			continue
		}
		// first try Reponse as it should be more frequent
		// is Response?
		if r.IsResponse() &&
			// same Address
			r.GetAddress() == reqADU.GetAddress() &&
			// request FunctionCode matches
			r.GetPduResponse().GetFunctionCode() == reqADU.GetPduRequest().GetFunctionCode() {
			return
		}

		// is Exception?
		if r.IsException() &&
			// same Address
			r.GetAddress() == reqADU.GetAddress() &&
			// FunctionExceptionCode matches reqADU FunctionCode
			r.GetPduResponseException().GetFunctionExceptionCode()&0x7F == reqADU.GetPduRequest().GetFunctionCode() {
			return
		}

		err = fmt.Errorf("cannot find ADU")
		return
	}

	return nil, 0, fmt.Errorf("no repy found")
}

// Size in [TimedBytes]
func (db *DissectorBuffer) Size() int {
	return len(db.TimedBytes)
}

func (db *DissectorBuffer) PrettyString() (s string) {
	for _, td := range db.GetTimedBytes() {
		s += fmt.Sprintf("[%v]", util.TimeBuilder(td.Time).Format(time.RFC3339))
		s += fmt.Sprintf("%02X", byte(td.Byte))
	}

	return
}
