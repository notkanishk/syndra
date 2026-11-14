package logstamp

import (
	"io"
	"log"
	"os"
	"time"
)

// Every log line in every Syndra service starts with one stamp: RFC 3339 in UTC
// with milliseconds and a literal Z, whatever TZ the deployment runs in. UTC is
// the interoperable default for logs: unambiguous in any reader's zone, sortable
// as text across services, and exact in unix time (date -d <stamp> +%s). Showing
// a time in the deployment's zone is the UI's job and the operator's tools'.
// Stored and sent instants are separate: timestamptz and RFC 3339 in JSON.
// Panics and stack traces are written by the runtime and carry no stamp.
//
// One copy per Go module (the backend and each add-on share no module); each
// copy has the same tests asserting the same format.
const stampLayout = "2006-01-02T15:04:05.000Z"

type stampWriter struct {
	w   io.Writer
	now func() time.Time
}

func (s stampWriter) Write(p []byte) (int, error) {
	if _, err := s.w.Write(append([]byte(s.now().UTC().Format(stampLayout)+" "), p...)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Install stamps the standard logger and anything that writes through it.
func Install() {
	log.SetFlags(0)
	log.SetOutput(stampWriter{os.Stderr, time.Now})
}
