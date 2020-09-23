# Introduction
This is a Modbus RTU sniffer, which can be utilized to passively read traffic
from a RS-485/422 serial line. It also provides the capability of scanning 
to identify the valid speed and serial frame configuration the Modbus RTU 
bus. 

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
Connect to a RS-485/422 serial bus (you will possibly need a hardware 
converter) with active traffic. Assuming port is `/dev/ttyUSB0`.

Scan port:
```
snifferModbusRTU -d /dev/ttyS0 -scan
...
```

Sniff traffic:
```
snifferModbusRTU -d /dev/ttyS0 -b 38400 -f 8N1
...
```

# License
See LICENSE file
