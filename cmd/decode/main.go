package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/andreaaizza/sniffer"

	"github.com/golang/protobuf/proto"
)

// Decodes Sniffer.ProtoBytesAndFlush() data from stdin, encoded as fmt.Sprintf("%d"), terminated with \n
// e.g. "10 68 10 32 10 30 8 1 120 153 99 130 1 12 8 153 214 149 252 5 16 248 ..."
func main() {
	file := os.Stdin
	reader := bufio.NewReader(file)
	if reader == nil {
		log.Panic()
	}
	//line := make([]byte, 1024*10)
	for {
		lineByte, err := reader.ReadBytes('\n')
		if err != nil {

			// done on EOF
			if err == io.EOF {
				log.Print("EOF reached. Done!")
				os.Exit(0)
			}

			log.Print(err)
			continue
		}
		line := strings.TrimSuffix(string(lineByte), "\n")

		// get single bytes
		ss := strings.Split(line, " ")
		b := make([]byte, 0)
		for _, s := range ss {
			i, err := strconv.Atoi(s)
			if err != nil {
				log.Panic(err)
			}
			b = append(b, byte(i))
		}

		r := sniffer.Results{}
		err = proto.Unmarshal(b, &r)
		if err != nil {
			log.Panic(err)
		}

		for _, rr := range r.GetResults() {
			fmt.Print(rr.PrettyString(), "\n")
		}
	}
}
