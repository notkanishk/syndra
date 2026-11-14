package logstamp

import (
	"io"
	"log"
	"os"
	"time"
)

// Every log line in every Syndra service starts with one stamp: RFC 3339 with
// milliseconds and an explicit offset, in the deployment's zone (time.Local,
// which follows TZ; unset is "Z"). It is unambiguous in any reader's zone, it
// sorts, and it converts exactly to unix time: date -d <stamp> +%s.
// Instants stored and sent are separate: timestamptz and RFC 3339 in JSON.
//
// One copy per Go module (the backend and each add-on share no module); each
// copy has the same test asserting the same format.
const stampLayout = "2006-01-02T15:04:05.000Z07:00"

type stampWriter struct {
	w   io.Writer
	now func() time.Time
}

func (s stampWriter) Write(p []byte) (int, error) {
	if _, err := s.w.Write(append([]byte(s.now().Format(stampLayout)+" "), p...)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Install stamps the standard logger and anything that writes through it.
func Install() {
	log.SetFlags(0)
	log.SetOutput(stampWriter{os.Stderr, time.Now})
}
