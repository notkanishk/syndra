// Command syndra-addon-conform says whether an add-on may be connected to
// Syndra (design §7 of openspec/changes/addon-contract). It carries the
// contract it was built from (addons/contract, embedded); there is no override.
//
//	syndra-addon-conform -url https://addon.example.org:8443 -target truenas -secret-file ./secret \
//	                     [-subject conformance-<run>] [-email conformance-<run>@example.org] [-mutate [-mutate-settings]] [-json report.json]
//	                     [-stop-app <command> -start-app <command> [-hang-app <command>] [-require-outage]] [-stop-addon <command> -start-addon <command>]
//
// Reads only unless -mutate: GET routes, /plan, and signed POST
// /operations/{id} for each operation the registry gives effect: read. Exit 0: every MUST holds. 1: a MUST failed.
// 2: the run could not start, or -mutate was refused. 130: interrupted, after
// putting back what the run had changed (a second interrupt kills at once).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	elevatedFile := flag.String("elevated-key-file", "", "with -mutate, a file holding account.purge's elevated_key: the kit purges its own account once it is off and creates it again (identity.not-reused); without it nothing is deleted")
	hook := func(name, usage string) *string {
		return flag.String(name, "", usage+" (run by sh -c; it returns once the change has happened)")
	}
	stopApp := hook("stop-app", "a command that makes the app unreachable from the add-on, for the outage checks")
	startApp := hook("start-app", "a command that makes the app reachable again")
	hangApp := hook("hang-app", "a command that makes the app hang: answer nothing, never refuse, as a blackholed address does; -start-app brings it back (outage.hang)")
	stopAddon := hook("stop-addon", "a command that stops the add-on, its disk kept, for outage.restart")
	startAddon := hook("start-addon", "a command that starts the add-on again")
	flag.BoolVar(&cfg.RequireOutage, "require-outage", false, "fail, rather than skip as not checked, the outage checks when -stop-app, -start-app or -hang-app is missing (CI passes it)")
	jsonOut := flag.String("json", "", "also write the report as JSON to this file; - writes JSON to stdout instead of text")
	flag.Parse()

	secret, err := os.ReadFile(*secretFile)
	if err != nil || strings.TrimSpace(string(secret)) == "" {
		fail("-secret-file must name a file holding the per-target secret")
	}
	cfg.Secret = []byte(strings.TrimSpace(string(secret)))
	if *elevatedFile != "" {
		key, err := os.ReadFile(*elevatedFile)
		if err != nil || strings.TrimSpace(string(key)) == "" {
			fail("-elevated-key-file must name a file holding account.purge's elevated_key")
		}
		cfg.ElevatedKey = strings.TrimSpace(string(key))
	}

	cfg.StopApp, cfg.StartApp, cfg.StopAddon, cfg.StartAddon = shell("-stop-app", *stopApp, hookTimeout), shell("-start-app", *startApp, hookTimeout),
		shell("-stop-addon", *stopAddon, hookTimeout), shell("-start-addon", *startAddon, hookTimeout)
	cfg.HangApp = shell("-hang-app", *hangApp, hookTimeout)

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

// hookTimeout bounds each hook: one that hangs fails, and the kit goes on to
// put back what it stopped.
const hookTimeout = 2 * time.Minute

// shell is the hook flag names: command run by sh -c, said and its output on
// stderr, killed after timeout; nil when there is no command.
func shell(flag, command string, timeout time.Duration) func(context.Context) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if noHooks != "" {
		fail(flag + ": " + noHooks)
	}
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		fmt.Fprintf(os.Stderr, "kit: running %s %s\n", flag, command)
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		ownGroup(cmd)
		cmd.WaitDelay = time.Second
		err := cmd.Run()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%s did not return within %s: %w", flag, timeout, err)
		}
		return err
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "syndra-addon-conform:", msg)
	os.Exit(2)
}
