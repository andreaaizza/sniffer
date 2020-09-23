package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/andreaaizza/sniffer/dissector"

	"github.com/golang/protobuf/proto"
)

func main() {
	// logging flags
	log.SetFlags(log.Lshortfile | log.Ltime)

	sData := os.Args[1]
	ss := strings.Split(sData, " ")
	b := make([]byte, 0)
	for _, s := range ss {
		i, err := strconv.Atoi(s)
		if err != nil {
			log.Panic(err)
		}
		b = append(b, byte(i))
	}

	r := dissector.Results{}
	err := proto.Unmarshal(b, &r)
	if err != nil {
		log.Panic(err)
	}

	for _, rr := range r.GetResults() {
		log.Print(rr.PrettyString())
	}
}
