.PHONY: proto

MOD=github.com/andreaaizza/sniffer


all: snifferModbusRTU decode

snifferModbusRTU: proto 
	go build ./cmd/snifferModbusRTU 

decode: proto 
	go build ./cmd/decode

clean: clean_proto
	-rm snifferModbusRTU decode

proto: logger/logger.pb.go dissector/dissector.pb.go sniffer.pb.go
logger/logger.pb.go: logger/logger.proto
	protoc --go_out=./ --go_opt=module=${MOD} logger/logger.proto
dissector/dissector.pb.go: dissector/dissector.proto
	protoc --go_out=./ --go_opt=module=${MOD} dissector/dissector.proto
sniffer.pb.go: dissector/dissector.pb.go sniffer.proto
	protoc --go_out=./ --go_opt=module=${MOD} sniffer.proto
clean_proto:
	-rm logger/logger.pb.go dissector/dissector.pb.go sniffer.pb.go

test: 
	go test -v ./...
