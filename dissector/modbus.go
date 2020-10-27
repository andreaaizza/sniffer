package dissector

import (
	fmt "fmt"
	"time"

	"github.com/andreaaizza/sniffer/util"
)

const (
	// ADUMinSize minium size of ADU in bytes
	ADUMinSize int = ADUSizePDUResponseException

	// ADUSizePDURequest size in bytes of a PDU request
	ADUSizePDURequest int = 8

	// ADUSizePDUResponseException size in bytes of a PDU exception
	ADUSizePDUResponseException int = 5
)

// NewADU builds an ADU from DissectorBuffer at position index. Returns err==nil on success
// Can be utilized to check if there is a ADU at specific position, also in cobination with
// IsRequest(), IsResponse(), IsException()
func NewADU(db *DissectorBuffer, index int) (adu *ADU, err error) {
	adu = &ADU{}

	// try building Request
	// 02040000000A703E 0204148003800380018001800180030037800380038003901F
	// 02 04 0000 000A 703E
	if index+ADUSizePDURequest > db.Size() {
		err = fmt.Errorf("buffer too short to try building PDURequest")
		return
	}
	// all PDURequest has 4 bytes in Data
	bytesData, err := db.bytes(index+2, 4)
	if err != nil {
		err = fmt.Errorf("cannot get bytes to build PDURquest daata")
		return
	}

	pduRequest := PDURequest{
		FunctionCode: db.TimedBytes[index+1].GetByte(),
		Data:         bytesData}

	adu.Address = db.TimedBytes[index].GetByte()
	adu.PDU = &ADU_PduRequest{PduRequest: &pduRequest}
	if err = adu.setCRC(db, index+ADUSizePDURequest-2); err != nil {
		return
	}
	adu.Time = db.TimedBytes[index].GetTime()

	if adu.IsRequest() {
		return
	}

	// try building response
	// 02040000000A703E 0204148003800380018001800180030037800380038003901F
	// 02 04 14 8003800380018001800180030037800380038003 901F
	dataLen := db.TimedBytes[index+2].GetByte()
	// db needs to have sufficient bytes
	if index+aduPDUResponseSizeFromDataLen(int(dataLen)) > db.Size() {
		err = fmt.Errorf("input buffer too short")
		return
	}
	bytesData, err = db.bytes(index+2, int(dataLen)+1)
	if err != nil {
		err = fmt.Errorf("cannot get bytes from buffer")
		return
	}
	pduResponse := PDUResponse{
		FunctionCode: db.TimedBytes[index+1].GetByte(),
		Data:         bytesData,
	}
	adu.Address = db.TimedBytes[index].GetByte()
	adu.PDU = &ADU_PduResponse{PduResponse: &pduResponse}
	if err = adu.setCRC(db, index+3+int(dataLen)); err != nil {
		return
	}
	adu.Time = db.TimedBytes[index].GetTime()

	if adu.IsResponse() {
		return
	}

	// try building an Exception
	//[1600175552]02042328000AFBB2[1600175552]02840232C1
	pduResponseException := PDUResponseException{
		FunctionExceptionCode: db.TimedBytes[index+1].GetByte(),
		ExceptionCode:         db.TimedBytes[index+2].GetByte(),
	}
	adu.Address = db.TimedBytes[index].GetByte()
	adu.PDU = &ADU_PduResponseException{PduResponseException: &pduResponseException}
	if err = adu.setCRC(db, index+3); err != nil {
		return
	}
	adu.Time = db.TimedBytes[index].GetTime()

	if adu.IsException() {
		return
	}

	err = fmt.Errorf("Cannot build any ADU")
	return
}

// IsRequest return true if ADU is a Modbus Request
func (adu *ADU) IsRequest() bool {
	return adu.checkCrc() == nil &&
		adu.GetPduRequest() != nil &&
		adu.GetPduRequest().GetFunctionCode()&0x80 == 0
}

// IsResponse return true if ADU is a Modbus Response without exceptions
func (adu *ADU) IsResponse() bool {
	return adu.checkCrc() == nil &&
		adu.GetPduResponse() != nil &&
		adu.GetPduResponse().GetFunctionCode()&0x80 == 0
}

// IsException return true if ADU is a Modbus Response with exceptions
func (adu *ADU) IsException() bool {
	return adu.checkCrc() == nil &&
		adu.GetPduResponseException() != nil &&
		adu.GetPduResponseException().GetFunctionExceptionCode()&0x80 == 0x80
}

// aduPDUResponseSizeFromDataLen size in bytes of a Response ADU, calculated from Data Len size
func aduPDUResponseSizeFromDataLen(l int) int {
	// 02040000000A703E 0204148003800380018001800180030037800380038003901F
	// 02 	04 	14 	8003800380018001800180030037800380038003 	901F
	// Ad 	Fu 	Data							CRC
	//		Len	Bytes(Len)

	// X    X       X  							XX -> 5
	return 5 + l
}

// Size returns ADU size in bytes, return 0 in case of error
func (adu *ADU) Size() int {
	if adu.GetPduResponseException() != nil {
		return ADUSizePDUResponseException
	} else if adu.GetPduRequest() != nil {
		return ADUSizePDURequest
	} else if pduResponse := adu.GetPduResponse(); pduResponse != nil {
		return 5 + int(pduResponse.Data[0])
	}
	return 0
}

// setCRC set CRC on DissectorBuffer position
func (adu *ADU) setCRC(db *DissectorBuffer, position int) (err error) {
	b, err := db.bytes(position, 2)
	if err != nil {
		return err
	}
	adu.Crc16 = uint32(b[1])<<8 + uint32(b[0])
	return
}

func (adu *ADU) PrettyString() (s string) {
	s = fmt.Sprintf("[%v] %02X", util.TimeBuilder(adu.GetTime()).Format(time.RFC3339Nano), adu.GetAddress())
	if adu.IsRequest() {
		pdu := adu.GetPduRequest()
		s += fmt.Sprintf("|REQ%02X|%02X", pdu.GetFunctionCode(), pdu.GetData())
	} else if adu.IsResponse() {
		pdu := adu.GetPduResponse()
		d := pdu.GetData()
		if len(d) > 8 {
			s += fmt.Sprintf("|RSP%02X|%02X....%02X", pdu.GetFunctionCode(), d[:4], d[len(d)-4:])
		} else {
			s += fmt.Sprintf("|RSP%02X|%02X", pdu.GetFunctionCode(), pdu.GetData())
		}
	} else if adu.IsException() {
		pdu := adu.GetPduResponseException()
		s += fmt.Sprintf("|EXC%02X|%02X", pdu.GetFunctionExceptionCode(), pdu.GetExceptionCode())
	} else {
		return "error. unknown PDU"
	}
	s += fmt.Sprintf("|%02X%02X", byte(adu.GetCrc16()), byte(adu.GetCrc16()>>8))
	return
}

// checkCrc checks ADU CRC and return nil if success
func (adu *ADU) checkCrc() (err error) {
	// get crc data for each PDU type
	pdu_crc_data := make([]byte, 0)
	if pduRequest := adu.GetPduRequest(); pduRequest != nil {
		pdu_crc_data = append([]byte{byte(adu.GetAddress()), byte(pduRequest.GetFunctionCode())},
			pduRequest.GetData()...)
	} else if pduResponse := adu.GetPduResponse(); pduResponse != nil {
		pdu_crc_data = append([]byte{byte(adu.GetAddress()), byte(pduResponse.GetFunctionCode())},
			pduResponse.GetData()...)
	} else if pduResponseException := adu.GetPduResponseException(); pduResponseException != nil {
		pdu_crc_data = []byte{byte(adu.GetAddress()), byte(pduResponseException.GetFunctionExceptionCode()), byte(pduResponseException.GetExceptionCode())}
	}
	crc := calcCRC(pdu_crc_data)
	//log.Printf("%02X %02X", crc, adu.Crc16) // LOG
	if byte(crc) == byte(adu.Crc16) && byte(crc>>8) == byte(adu.Crc16>>8) {
		return nil
	}
	return fmt.Errorf("Invalid CRC")
}

func (adu *ADU) GetTimeTime() time.Time {
	return util.TimeBuilder(adu.GetTime())
}
