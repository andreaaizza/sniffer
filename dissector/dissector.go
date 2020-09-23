package dissector

import (
	"sync"

	"github.com/andreaaizza/sniffer/logger"
	"github.com/andreaaizza/sniffer/util"

	"fmt"
	"log"
	"time"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
)

const (
	// DBMaxSizeWithoutNotify if more DataUnits than this in the buffer, then notity
	DBMaxSizeWithoutNotify = 4096

	// FlushAfterSecondsModbusRTU flush data if older than this [seconds]
	FlushAfterSecondsModbusRTU = 10
)

type Dissector struct {
	DissectorBuffer
	Consumer   chan logger.DataUnit
	Results    Results
	resultsMux sync.Mutex

	stop chan struct{}

	FlushAfterSeconds int
}

// New builds new dissector and starts waiting for data.
// DataUnits should be sent to GetConsumer() channel. Results can be fetched with GetResults(). Should be closed with Close() at the end.
func New() *Dissector {
	d := &Dissector{
		DissectorBuffer: DissectorBuffer{},
		Consumer:        make(chan logger.DataUnit),
		Results:         Results{},

		stop: make(chan struct{}, 0),

		FlushAfterSeconds: FlushAfterSecondsModbusRTU,
	}

	go func() {
		for {
			select {
			case <-d.stop:
				return
			case du := <-d.Consumer:
				d.loadDataUnit(du)

				// dissect after each packet recevied
				d.dissect()
			}
		}
	}()
	return d
}

// Close closes
func (d *Dissector) Close() {
	close(d.stop)
}

// GetConsumer return the channel to send DataUnits to
func (d *Dissector) GetConsumer() chan logger.DataUnit {
	return d.Consumer
}

// ProtoBytes extracts results as protobuf Marshalled bytes
func (d *Dissector) ProtoBytes() (b []byte, err error) {
	b, err = proto.Marshal(&d.Results)
	if err != nil {
		return
	}
	return
}

// FlushResults clear results queue
func (d *Dissector) FlushResults() {
	d.resultsMux.Lock()
	d.Results.Results = d.Results.Results[:0]
	d.resultsMux.Unlock()
}

// loadDataUnit pushes DataUnit to dissector
func (d *Dissector) loadDataUnit(du logger.DataUnit) {
	for _, dByte := range du.Data {
		d.TimedBytes = append(d.TimedBytes, &TimedByte{Time: du.Time, Byte: uint32(dByte)})
	}
}

// flushOldData flushes data if too old
func (d *Dissector) flushOldData() {
	if d.FlushAfterSeconds == 0 {
		return
	}
	t := time.Now().UTC()
	for i := len(d.TimedBytes) - 1; i > 0; i-- {
		td := util.TimeBuilder(*d.TimedBytes[i].GetTime())
		if t.After(td.Add(time.Duration(d.FlushAfterSeconds) * time.Second)) {
			//log.Print("Flushing dissector buffer: ", d.TimedBytes[i]) //LOG
			d.remove(i, 1)
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

		// flush old data
		d.flushOldData()

		if d.Size() > DBMaxSizeWithoutNotify {
			log.Printf("DissectorBuffer too big. Size=%d. Content: %s", d.Size(), d.PrettyString())
		}
	}
}

// tries finding req->resp couple, returns true if success
func (d *Dissector) dissectRound() bool {
	req := ADU{}
	// cycle thru each byte and search for a valid Request
	for reqIndex, _ := range d.TimedBytes {
		if req.NewADU(&d.DissectorBuffer, reqIndex) == nil && req.IsRequest() {

			// find subsequent Response or Exeption
			resp, respIndex, err := d.findModbusReply(reqIndex)
			if err == nil {
				// found couple Request --> { Response, Exception }

				// LOG
				//log.Printf("MATCH: [%d]%s -> [%d]%s", reqIndex, req.PrettyString(), respIndex, resp.PrettyString()) // LOG
				//log.Printf("[%03d]%s -> [%03d]%s\n", req.Size(), req.PrettyString(), resp.Size(), resp.PrettyString()) // LOG

				// push to results
				d.resultsMux.Lock()
				d.Results.Results = append(d.Results.Results, &Result{Request: &req, Reponse: resp})
				d.resultsMux.Unlock()

				// REMOVE
				d.remove(respIndex, resp.Size()) // first remove the newest bytes: the response, to not interfere with request (olded) removal
				d.remove(reqIndex, req.Size())
				return true
			}
		}
	}
	return false
}

// remove removes `size` data from buffer at `start` position
func (db *DissectorBuffer) remove(start int, size int) {
	db.TimedBytes = append(db.TimedBytes[:start], db.TimedBytes[start+size:]...)
}

/*
LB errors: [1600174478]020300090001543B[1600174478]02830230F1[1600174478]02030000000AC5FE[1600174478]02830230F1[1600174480]020300090001543B[1600174480]02830230F1[1600174480]02030000000AC5FE[1600174480]02830230F1[1600174482]020300090001543B[1600174482]02830230F1[1600174482]02030000000AC5FE[1600174482]02830230F1[1600174484]020300090001543B[1600174484]02830230F1[1600174484]02030000000AC5FE[1600174484]02830230F1
*/

// bytes returns `size` []bytes from `start`
func (db DissectorBuffer) bytes(start int, size int) (b []byte, err error) {
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
	r = &ADU{}
	reqADU := &ADU{}
	if reqADU.NewADU(db, mdReqADUindex) != nil || !reqADU.IsRequest() {
		err = fmt.Errorf("should find an ADU with valid PDURequest at index ", i)
		log.Print(err)
		return

	}
	for i = mdReqADUindex + reqADU.Size(); i < db.Size()-ADUMinSize; i++ {
		if r.NewADU(db, i) != nil {
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
func (db DissectorBuffer) Size() int {
	return len(db.TimedBytes)
}

func (db DissectorBuffer) PrettyString() (s string) {
	t := google_protobuf.Timestamp{}
	t.Seconds = 0
	t.Nanos = 0
	for _, td := range db.GetTimedBytes() {
		if td.Time.Seconds > t.Seconds && td.Time.Nanos > t.Nanos {
			t = *td.Time
			s += fmt.Sprintf("[%v]", util.TimeBuilder(*td.Time).Format(time.RFC3339))
		}
		s += fmt.Sprintf("%02X", byte(td.Byte))
	}

	return
}
