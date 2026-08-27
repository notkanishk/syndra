package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Guards over the ROUTER and the handler package's own conventions, checked
// against the source rather than trusted.
//
// Each rule below was already true, and each was already written down — in
// CLAUDE.md, in `deps.go`'s own comment, in the doc on `decodeJSONLenient`.
// None of them was checked, which means none of them was a rule: it was a habit
// that had held so far. A habit does not survive the next hurried afternoon,
// and the one that matters here is the gate on a route.

func handlerSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read handler package: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		out[name] = string(b)
	}
	return out
}

var routeLine = regexp.MustCompile(`mux\.HandleFunc\("(\w+) ([^"]+)",\s*(.+)\)\n`)

// TestEveryRouteCarriesAGate is the one guard here that is about security
// rather than coherence.
//
// A route registered without a gate is not a style mistake — it is an endpoint
// serving whoever finds it, and nothing about the line it is written on looks
// different from the 130 lines around it that are safe. The gates are named
// explicitly rather than matched by shape ("contains 'Auth'"), because a
// misspelled wrapper that happened to contain the word would pass a shape test.
func TestEveryRouteCarriesAGate(t *testing.T) {
	gates := []string{
		"withUserAuth",               // any signed-in person
		"withOperatorAuth",           // admin only
		"withSelfOrOperatorAuth",     // the subject themselves, or an admin
		"withAPIKeyAuth",             // a service, by shared key
		"withZitadelActionSignature", // Zitadel, by request signature
	}
	// `/healthz` answers before anything is configured and must stay reachable
	// by the container runtime, which carries no credential of any kind.
	ungated := map[string]bool{"GET /healthz": true}

	src := handlerSources(t)["router.go"]
	var offenders []string
	for _, m := range routeLine.FindAllStringSubmatch(src, -1) {
		route := m[1] + " " + m[2]
		if ungated[route] {
			continue
		}
		found := false
		for _, g := range gates {
			if strings.Contains(m[3], g+"(") {
				found = true
				break
			}
		}
		if !found {
			offenders = append(offenders, route)
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("routes with no recognised auth gate:\n  %s", strings.Join(offenders, "\n  "))
	}
}

// TestEveryMutationDecodesStrictly holds the rule CLAUDE.md states and nothing
// enforced: a mutation endpoint rejects unknown fields.
//
// A lenient decode silently drops a field the caller believed it sent. On a
// mutation that means the caller thinks it asked for something the server never
// read — and on the one endpoint in this package that removes access, that had
// actually happened.
//
// Handlers that only delegate are followed one hop, because a two-line handler
// that hands off to a shared body is the normal shape here and not an evasion.
func TestEveryMutationDecodesStrictly(t *testing.T) {
	// Zitadel owns this payload and may extend it; rejecting a field a newer
	// Zitadel adds would break the claim path on THEIR upgrade, at a moment
	// nobody here chose. `decodeJSONLenient` keeps the trailing-token guard, so
	// the laxity is on the field set only.
	argued := map[string]string{
		"HandleActionInject": "Zitadel Actions v2 owns this body shape",
	}

	src := handlerSources(t)
	bodies := map[string]string{}
	for _, s := range src {
		for _, m := range regexp.MustCompile(`\nfunc (\w+)\(`).FindAllStringSubmatchIndex(s, -1) {
			bodies[s[m[2]:m[3]]] = funcBody(s, m[1])
		}
	}

	var offenders []string
	for _, m := range routeLine.FindAllStringSubmatch(src["router.go"], -1) {
		if m[1] == "GET" || m[1] == "HEAD" {
			continue
		}
		names := regexp.MustCompile(`\b([Hh]andle\w+)\b`).FindAllStringSubmatch(m[3], -1)
		if len(names) == 0 {
			continue
		}
		fn := names[len(names)-1][1]
		if _, ok := argued[fn]; ok {
			continue
		}
		body, ok := bodies[fn]
		if !ok {
			continue
		}
		// One delegation hop: a handler whose whole job is to call a shared
		// implementation with a constant.
		if !strings.Contains(body, "r.Body") && !strings.Contains(body, "decodeJSON") {
			for _, call := range regexp.MustCompile(`\b(\w+)\(w, r[,)]`).FindAllStringSubmatch(body, -1) {
				if inner, ok := bodies[call[1]]; ok {
					body += inner
				}
			}
		}
		readsBody := strings.Contains(body, "r.Body") || strings.Contains(body, "decodeJSON")
		if readsBody && !strings.Contains(body, "decodeJSONStrict") {
			offenders = append(offenders, m[1]+" "+m[2]+" -> "+fn)
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("mutation routes reading a body without decodeJSONStrict:\n  %s", strings.Join(offenders, "\n  "))
	}
}

// TestNoHandlerBypassesItsOwnSeam.
//
// `deps.go` exists so a handler can be tested without the world behind it. A
// handler that calls the service directly while a seam for that exact function
// sits in `deps.go` is untestable AND makes the seam dead code that reads as
// live — the next person substitutes it, sees no effect, and goes looking for a
// bug in their test.
func TestNoHandlerBypassesItsOwnSeam(t *testing.T) {
	src := handlerSources(t)
	seam := regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*((?:\w+\.)+\w+)\s*$`)
	targets := map[string]string{} // services.Foo -> seam name
	for _, m := range seam.FindAllStringSubmatch(src["deps.go"], -1) {
		targets[m[2]] = m[1]
	}

	var offenders []string
	for name, s := range src {
		if name == "deps.go" {
			continue
		}
		for call, varName := range targets {
			for _, idx := range regexp.MustCompile(regexp.QuoteMeta(call)+`\(`).FindAllStringIndex(s, -1) {
				line := strings.Count(s[:idx[0]], "\n") + 1
				offenders = append(offenders,
					name+":"+itoa(line)+" calls "+call+" directly; seam `"+varName+"` is right there")
			}
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("dependency seams bypassed:\n  %s", strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// funcBody returns the braced body of the function whose `func` keyword starts
// at `from`.
func funcBody(s string, from int) string {
	open := strings.Index(s[from:], "{")
	if open < 0 {
		return ""
	}
	open += from
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open : i+1]
			}
		}
	}
	return s[open:]
}
