package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// The payloads below are the SHAPES `nas.example.org` answered with on
// 2026-08-24, TrueNAS-25.10.5, trimmed to the fields this add-on reads and with
// the identifying values replaced. What is preserved is what matters and what
// keeps going wrong: the TYPES.
//
// `activity.get` was broken for the whole life of this add-on because
// `message_timestamp` is an integer and was decoded as a string. Nothing
// noticed, because the recorded fixture stored key NAMES. These tests exist so
// the next type change fails here instead of in production.
const (
	realSystemInfo = `{"hostname":"truenas","version":"25.10.5","uptime_seconds":321588.114393215,` +
		`"system_serial":"0123456789","license":null,"system_manufacturer":"QEMU","cores":16}`

	// `datetime` is `{"$date": <millis>}`, and `formatted` carries HTML.
	realAlertList = `[{"klass":"SMARTUncorrectedErrors","level":"WARNING","dismissed":false,` +
		`"formatted":"1 uncorrectable errors reported for sde (SERIAL0000).","text":null,` +
		`"datetime":{"$date":1783335199000}},` +
		`{"klass":"RESTAPIUsage","level":"WARNING","dismissed":false,` +
		`"formatted":"The deprecated REST API was used 1 times<br>from 198.51.100.16.","text":null,` +
		`"datetime":{"$date":1787400000000}}]`

	realPoolQuery = `[{"name":"pool0","status":"ONLINE","healthy":true,"warning":false,` +
		`"free":38978043379712,"allocated":999512211456,"size":39977555591168,"topology":{}}]`

	realServiceQuery = `[{"service":"cifs","state":"RUNNING","enable":true},` +
		`{"service":"nfs","state":"RUNNING","enable":true},` +
		`{"service":"ftp","state":"STOPPED","enable":false}]`
)

// methodRPC answers per method. A named method is REFUSED by the target — an
// answered "no", which is what a restricted API key produces and the failure
// per-source degradation is for. A dead socket is a different failure with a
// different blast radius, and `deadAfter` produces that one.
type methodRPC struct {
	answers   map[string]string
	fails     map[string]bool
	deadAfter string
	dead      bool
	calls     []string
}

func (m *methodRPC) Call(method string, _ int64, _ any) (json.RawMessage, error) {
	m.calls = append(m.calls, method)
	if m.dead {
		return nil, errors.New("websocket: connection closed")
	}
	if method == m.deadAfter {
		m.dead = true
		return nil, errors.New("websocket: connection closed")
	}
	if m.fails[method] {
		return json.RawMessage(`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,` +
			`"message":"[EACCES] Not authorized","data":{"error":13,"errname":"EACCES"}}}`), nil
	}
	body, ok := m.answers[method]
	if !ok {
		body = "null"
	}
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":` + body + `}`), nil
}
func (m *methodRPC) Ping() (string, error) { return "pong", nil }
func (m *methodRPC) Close() error          { return nil }

func systemHealthServer(t *testing.T, fails ...string) (*server, *methodRPC) {
	t.Helper()
	answering := &methodRPC{
		answers: map[string]string{
			"system.info":    realSystemInfo,
			"alert.list":     realAlertList,
			"pool.query":     realPoolQuery,
			"service.query":  realServiceQuery,
			"core.ping":      `"pong"`,
			"system.version": `"25.10.5"`,
		},
		fails: map[string]bool{},
	}
	for _, f := range fails {
		answering.fails[f] = true
	}
	n := newNAS(func() (rpc, error) { return answering, nil }, []string{"25.10"})
	n.probed = true
	return &server{nas: n}, answering
}

func TestSystemHealthReadsWhatTheTargetActuallyAnswers(t *testing.T) {
	s, _ := systemHealthServer(t)

	res, code, err := s.targetHealth(OperationRequest{})
	if err != nil || code != http.StatusOK {
		t.Fatalf("want 200, got %d: %v", code, err)
	}
	h := res.Health
	if h == nil {
		t.Fatal("no health report")
	}

	if h.System == nil || h.System.Hostname != "truenas" || h.System.Version != "25.10.5" {
		t.Fatalf("system not decoded: %+v", h.System)
	}
	// The uptime is a FLOAT on the wire. An int64 field decodes it as an error
	// and takes the whole read down with it.
	if h.System.UptimeSeconds < 321588 || h.System.UptimeSeconds > 321589 {
		t.Fatalf("uptime not decoded from a float: %v", h.System.UptimeSeconds)
	}

	if len(h.Alerts) != 2 {
		t.Fatalf("want both alerts, got %d", len(h.Alerts))
	}
	if h.Alerts[0].Text != "1 uncorrectable errors reported for sde (SERIAL0000)." {
		t.Fatalf("alert text: %q", h.Alerts[0].Text)
	}
	// `{"$date": ms}`, not a number and not a string.
	if h.Alerts[0].At != "2026-07-06T10:53:19Z" {
		t.Fatalf("alert timestamp not decoded from the $date wrapper: %q", h.Alerts[0].At)
	}
	if strings.Contains(h.Alerts[1].Text, "<") {
		t.Fatalf("markup reached a caller: %q", h.Alerts[1].Text)
	}
	if h.Alerts[1].Text != "The deprecated REST API was used 1 times from 198.51.100.16." {
		t.Fatalf("a stripped tag must leave a word boundary: %q", h.Alerts[1].Text)
	}

	if len(h.Pools) != 1 || h.Pools[0].Name != "pool0" || !h.Pools[0].Healthy {
		t.Fatalf("pools: %+v", h.Pools)
	}
	if h.Pools[0].SizeBytes != 39977555591168 {
		t.Fatalf("pool size lost precision: %d", h.Pools[0].SizeBytes)
	}
	if len(h.Services) != 3 {
		t.Fatalf("want every service, got %d", len(h.Services))
	}
	if len(h.Degraded) != 0 {
		t.Fatalf("nothing failed, so nothing is degraded: %v", h.Degraded)
	}
}

// The chassis serial, the license and the manufacturer answer no question
// Syndra asks, and this is the only place that decides they do not travel.
func TestSystemHealthDoesNotForwardHardwareIdentifiers(t *testing.T) {
	s, _ := systemHealthServer(t)

	res, _, err := s.targetHealth(OperationRequest{})
	if err != nil {
		t.Fatalf("targetHealth: %v", err)
	}
	body, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"0123456789", "system_serial", "license", "QEMU", "topology"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("%q reached the response: %s", forbidden, body)
		}
	}
}

// One source failing tells an operator nothing about the other three, which are
// usually the ones that would have explained it.
func TestOneFailedSourceDegradesByNameAndKeepsTheRest(t *testing.T) {
	s, _ := systemHealthServer(t, "alert.list")

	res, code, err := s.targetHealth(OperationRequest{})
	if err != nil || code != http.StatusOK {
		t.Fatalf("a partial read is still an answer, got %d: %v", code, err)
	}
	h := res.Health
	if len(h.Degraded) != 1 || h.Degraded[0] != "alerts" {
		t.Fatalf("the failed source must be NAMED, got %v", h.Degraded)
	}
	if h.Alerts != nil {
		t.Fatalf("an unread source must be absent, not empty: %v", h.Alerts)
	}
	if h.System == nil || len(h.Pools) != 1 || len(h.Services) != 3 {
		t.Fatal("the sources that answered must survive one that did not")
	}
}

// A DEAD SESSION is not one bad source. The first transport failure drops the
// session, and the reconnect cooldown — the thing that stops a burst of retries
// locking the API key out — refuses the redial for the next fifteen seconds, so
// every read after it in the same pass fails too.
//
// Recorded rather than fixed. Within one `health.get` a dead socket genuinely
// means the target is not answering, and shortening the cooldown to make this
// card prettier would trade the lockout defence for a cosmetic one. What the
// surface must not do is imply the unread sources were read, and it does not:
// they are named in `degraded`.
func TestADeadSessionDegradesEverythingAfterIt(t *testing.T) {
	s, answering := systemHealthServer(t)
	answering.deadAfter = "alert.list"

	res, code, err := s.targetHealth(OperationRequest{})
	if err != nil || code != http.StatusOK {
		t.Fatalf("the source that answered first still answered, got %d: %v", code, err)
	}
	h := res.Health
	if h.System == nil {
		t.Fatal("the read before the socket died must survive")
	}
	if len(h.Degraded) != 3 {
		t.Fatalf("every read after a dead socket is unread and must say so, got %v", h.Degraded)
	}
}

// All four failing is an unreachable target, not a health report with four
// holes in it.
func TestEveryFailedSourceIsAnUnreachableTarget(t *testing.T) {
	s, _ := systemHealthServer(t, "system.info", "alert.list", "pool.query", "service.query")

	_, code, err := s.targetHealth(OperationRequest{})
	if err == nil {
		t.Fatal("want a refusal")
	}
	if code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", code)
	}
}

// A shape check, not a value list: a release that adds an NTSTATUS keeps
// working, and one that starts writing prose into the field does not.
func TestStatusTokenAdmitsOnlyTokens(t *testing.T) {
	for _, ok := range []string{"NT_STATUS_NO_SUCH_USER", "OK", "ERR_2"} {
		if statusToken(ok) != ok {
			t.Fatalf("%q must be admitted", ok)
		}
	}
	for _, bad := range []string{
		"", "no such user", "NT_STATUS: password Hunter2", "nt_status_ok",
		strings.Repeat("A", 65),
	} {
		if got := statusToken(bad); got != "" {
			t.Fatalf("%q must be dropped, got %q", bad, got)
		}
	}
}
