// Command reference-addon serves the reference add-on on a local address, for
// the conformance kit and for an author reading the guide.
//
//	reference-addon -listen 127.0.0.1:8443 -target reference -secret-file ./secret
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"syndra/addons/reference"
)

type names []string

func (n *names) String() string     { return strings.Join(*n, ",") }
func (n *names) Set(v string) error { *n = append(*n, strings.Split(v, ",")...); return nil }

func main() {
	stampLogs()
	listen := flag.String("listen", "127.0.0.1:8443", "address to serve on")
	target := flag.String("target", "reference", "the target name: the salt of the key derivation")
	secretFile := flag.String("secret-file", "", "file holding the per-target secret (trimmed UTF-8)")
	var defects, features names
	flag.Var(&features, "features", fmt.Sprintf("serve an optional feature (repeatable, or comma-separated): %v", reference.OptionalFeatures))
	flag.Var(&defects, "defect", fmt.Sprintf("switch on a deliberate contract violation, for the kit's self-tests only (repeatable): %v", reference.Defects))
	flag.Parse()

	raw, err := os.ReadFile(*secretFile)
	secret := strings.TrimSpace(string(raw))
	if err != nil || secret == "" {
		log.Fatalf("-secret-file must name a file holding the per-target secret")
	}
	set, err := reference.ParseDefects(defects)
	if err != nil {
		log.Fatal(err)
	}
	if len(set) > 0 {
		log.Printf("WARNING: deliberately violating the contract (%s); never connect this to Syndra", defects.String())
	}
	addon, err := reference.New(reference.Config{Target: *target, Secret: []byte(secret), Features: features, Defects: set})
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              *listen,
		Handler:           addon.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{addon.Certificate()}},
	}
	log.Printf("reference add-on for target %q on https://%s", *target, *listen)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}
