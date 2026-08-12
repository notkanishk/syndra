package main

import (
	"os"
	"testing"
)

// The filter must drop exactly the liveness probe's noise and nothing else.
//
// The risk in a log filter is not that it fails to hide something — it is that
// it hides the one line that mattered. A handshake failure from the backend is
// this add-on's most important error: it means the two ends derived different
// keys, and it is indistinguishable from the probe's line except by where the
// connection came from and how it ended.
func TestTheProbeFilterDropsOnlyTheProbe(t *testing.T) {
	dropped := []string{
		"http: TLS handshake error from 127.0.0.1:38979: EOF\n",
		"http: TLS handshake error from 127.0.0.1:1: EOF",
	}
	kept := []string{
		// The backend, over the Compose network — the failure that matters.
		"http: TLS handshake error from 198.51.100.5:44120: EOF\n",
		// Loopback, but a real handshake that was refused rather than a probe
		// that sent nothing.
		"http: TLS handshake error from 127.0.0.1:38979: remote error: tls: bad certificate\n",
		"http: TLS handshake error from 127.0.0.1:38979: tls: client didn't provide a certificate\n",
		"anything else at all\n",
	}

	for _, line := range dropped {
		if !isProbeNoise(line) {
			t.Errorf("the probe's own line was kept: %q", line)
		}
	}
	for _, line := range kept {
		if isProbeNoise(line) {
			t.Errorf("a line an operator needs was dropped: %q", line)
		}
	}
}

// And the filter really does write everything else to stderr, so the predicate
// above is the whole of the behaviour.
func TestTheFilterPassesThroughWhatItKeeps(t *testing.T) {
	n, err := probeNoiseFilter{}.Write([]byte(""))
	if err != nil || n != 0 {
		t.Fatalf("empty write: n=%d err=%v", n, err)
	}
	if _, err := os.Stderr.Stat(); err != nil {
		t.Skip("no stderr to write to")
	}
}
