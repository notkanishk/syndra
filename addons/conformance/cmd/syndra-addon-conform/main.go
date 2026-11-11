// Command syndra-addon-conform says whether an add-on may be connected to
// Syndra (design §7 of openspec/changes/addon-contract). It carries the
// contract it was built from (addons/contract, embedded); there is no override.
//
//	syndra-addon-conform -url https://addon.example.org:8443 -target truenas -secret-file ./secret \
//	                     [-subject conformance-<run>] [-email conformance-<run>@example.org] [-mutate [-mutate-settings]] [-json report.json]
//
// Reads only unless -mutate. Exit 0: every MUST holds. 1: a MUST failed.
// 2: the run could not start, or -mutate was refused. 130: interrupted, after
// putting back what the run had changed (a second interrupt kills at once).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"syndra/addons/conformance"
)

func main() {
	var cfg conformance.Config
	flag.StringVar(&cfg.URL, "url", "", "the add-on's base URL, https://host:port")
	flag.StringVar(&cfg.Target, "target", "", "the target name the add-on was configured with (the derivation's salt)")
	secretFile := flag.String("secret-file", "", "file holding the per-target secret (trimmed UTF-8)")
	flag.StringVar(&cfg.Subject, "subject", "", "the throwaway subject for -mutate, beginning conformance- (default conformance-<UTC time>)")
	flag.StringVar(&cfg.Email, "email", "", "the subject's email, beginning conformance- (default <subject>@example.org)")
	flag.BoolVar(&cfg.Mutate, "mutate", false, "send writes: create, converge and finally disable one throwaway account")
	flag.StringVar(&cfg.Item, "item", "", "with -mutate and feature calls, an item the kit may check out to its subject with a physical operation, and leave checked out")
	flag.BoolVar(&cfg.MutateSettings, "mutate-settings", false, "with -mutate, also write the first target-wide setting's current value back")
	jsonOut := flag.String("json", "", "also write the report as JSON to this file; - writes JSON to stdout instead of text")
	flag.Parse()

	secret, err := os.ReadFile(*secretFile)
	if err != nil || strings.TrimSpace(string(secret)) == "" {
		fail("-secret-file must name a file holding the per-target secret")
	}
	cfg.Secret = []byte(strings.TrimSpace(string(secret)))

	// The first SIGINT or SIGTERM stops the run between requests and lets it
	// put back what it changed; the second gets the default: exit now.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() { <-ctx.Done(); stop() }()

	report, err := conformance.Run(ctx, cfg)
	if report == nil {
		fail(err.Error())
	}
	if *jsonOut == "-" {
		_ = report.WriteJSON(os.Stdout)
	} else {
		report.WriteText(os.Stdout)
	}
	if *jsonOut != "" && *jsonOut != "-" {
		f, ferr := os.Create(*jsonOut)
		if ferr == nil {
			ferr = report.WriteJSON(f)
			_ = f.Close()
		}
		if ferr != nil {
			fail(ferr.Error())
		}
	}
	switch {
	case errors.Is(err, conformance.ErrInterrupted):
		fmt.Fprintln(os.Stderr, "syndra-addon-conform:", err)
		os.Exit(130)
	case err != nil:
		fail("refused: " + err.Error())
	case report.Failed():
		os.Exit(1)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "syndra-addon-conform:", msg)
	os.Exit(2)
}
