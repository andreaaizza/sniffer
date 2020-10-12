# Introduction
This is a Modbus RTU sniffer, which can be utilized to passively read traffic from a RS-485 half-duplex or full duplex serial line (transmit and receive on the same single serial line). It also provides the capability of scanning to identify the valid speed and serial frame configuration the Modbus RTU bus; a scan is successful in case both a request and the corresponding response are found. 

A scan with `debug` flag can be utilized to read serial data from duplex lines, but niether Modbus data is not decoded nor scan succeeds.

This is tested on `linux` (also `GOARCH=arm`), can be extended to `windows`. 

# Install
It requires [Go](https://golang.org/doc/install) and [Protobuf](https://developers.google.com/protocol-buffers/docs/downloads).
```
git clone github.com/andreaaizza/sniffer
cd sniffer 
make proto
go install ./cmd/snifferModbusRTU
```

# Examples
Connect to a RS-485 serial (you might possibly need a hardware converters) with active traffic.

## Scanner
Scan half-duplex port `/dev/ttyUSB0` for all speed/frame combinations:
```
snifferModbusRTU -d1 /dev/ttyUSB0 -scan
```

To scan duplex ports you need to add `-duplex` and the second port `-d2 dev/ttyUSB1`. E.g.:
```
snifferModbusRTU -d1 /dev/ttyUSB0 -d2 /dev/ttyUSB1 -duplex -scan
```

Restrict scan to just `9600` bps configurations (add `-duplex` if you wish):
```
snifferModbusRTU -d1 /dev/ttyUSB0 -b 9600 -scan
```
You might use frame format restriction e.g. `-f 8E1`.

## Sniffer
Sniff traffic from half-duplex port `/dev/ttyUSB0` with baud `38400` and frameformat `8N1`:
```
snifferModbusRTU -d1 /dev/ttyUSB0 -b 38400 -f 8N1
```
Sniff traffic from duplex port `/dev/ttyUSB0` (tx, requests), `/dev/ttyUSB1` (rx, responses/exceptions)  with baud `9600` and frameformat `8N1`:
```
snifferModbusRTU -d1 /dev/ttyUSB0 -d2 /dev/ttyUSB1 -duplex
```

# License
See LICENSE file
