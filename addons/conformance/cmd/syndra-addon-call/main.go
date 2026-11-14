// Command syndra-addon-call sends one signed request to an add-on, for the
// staging window where the backend is stopped (scripts/addon-quiet-run.sh
// --writes). It uses the conformance kit's own transport, never a copy.
//
//	syndra-addon-call -url https://host:8443 -target truenas -secret-file F <METHOD> <path> [json-body]
//
// Allowed: PUT/GET /settings/<name>, POST /operations/account.release for a
// conformance- subject, GET /health /capabilities /subjects /settings. Nothing
// else is sent; deleting stays with a person on the NAS. Prints the status and
// the body. Exit 0 on 2xx, 1 on any other answer, 2 when refused or not sent.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"syndra/addons/conformance"
)

func main() {
	var cfg conformance.CallConfig
	flag.StringVar(&cfg.URL, "url", "", "the add-on's base URL, https://host:port")
	flag.StringVar(&cfg.Target, "target", "", "the target name the add-on was configured with")
	secretFile := flag.String("secret-file", "", "file holding the per-target secret (trimmed UTF-8)")
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 || len(args) > 3 {
		fail("usage: syndra-addon-call -url U -target T -secret-file F <METHOD> <path> [json-body]")
	}
	secret, err := os.ReadFile(*secretFile)
	if err != nil || strings.TrimSpace(string(secret)) == "" {
		fail("-secret-file must name a file holding the per-target secret")
	}
	cfg.Secret = []byte(strings.TrimSpace(string(secret)))
	var body []byte
	if len(args) == 3 {
		body = []byte(args[2])
	}
	status, reply, err := conformance.Call(context.Background(), cfg, strings.ToUpper(args[0]), args[1], body)
	if err != nil {
		fail(err.Error())
	}
	fmt.Printf("%d\n%s\n", status, reply)
	if status/100 != 2 {
		os.Exit(1)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "syndra-addon-call:", msg)
	os.Exit(2)
}
