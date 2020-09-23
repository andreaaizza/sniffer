package util

import (
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
)

func TimeBuilder(timestamp timestamp.Timestamp) time.Time {
	return time.Unix(timestamp.GetSeconds(), int64(timestamp.GetNanos()))
}

func TimestampBuilder(time time.Time) timestamp.Timestamp {
	return timestamp.Timestamp{
		Seconds: time.Unix(),
		Nanos:   int32(time.UnixNano() - time.Unix()*1e9),
	}
}
