.PHONY: proto

all: snifferModbusRTU decode

snifferModbusRTU: proto 
	go build ./cmd/snifferModbusRTU 

decode: proto 
	go build ./cmd/decode

clean: clean_proto
	-rm snifferModbusRTU decode

proto: logger/logger.pb.go dissector/dissector.pb.go
logger/logger.pb.go: logger/logger.proto
	protoc --go_out=./ logger/logger.proto
dissector/dissector.pb.go: dissector/dissector.proto
	protoc --go_out=./ dissector/dissector.proto
clean_proto:
	-rm logger/logger.pb.go dissector/dissector.pb.go

test: 
	go test -v ./...
