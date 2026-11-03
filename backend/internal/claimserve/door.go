package claimserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"syndra/internal/claimtel"
)

// Door is the claims target's HTTP route: signature, decode, answer, count.
// The same code serves the claims process (cmd/claims, :8090) and the
// backend's in-process fallback (:8080); only the hooks differ, so the two
// doors cannot drift apart.
type Door struct {
	Plane func() *Plane
	// Keys is every key a call may be signed with, keyring first.
	Keys func() []SlotKey
	// DevMode passes unsigned calls when no key is configured at all
	// (ZITADEL_DOMAIN unset). Off, no key refuses every call (audit C1).
	DevMode func() bool
	// Record hears every finished call, exactly once.
	Record func(claimtel.Call)
	// OnMiss hears a person with no facts for the project: recompile them.
	OnMiss func(userID string)
	// OnMismatch hears a call no key verified: read the keyring again.
	OnMismatch func()
	// Debug, when set, is asked about each answered call (debug mode).
	Debug Debugger
}

// Debugger is debug mode as the data plane sees it.
type Debugger interface {
	Wants(clientID, userID string) bool
	Capture(DebugCapture)
}

// DebugCapture is one watched sign-in: what Zitadel sent and Syndra answered.
type DebugCapture struct {
	At        time.Time       `json:"at"`
	Function  string          `json:"function"`
	ClientID  string          `json:"client_id"`
	UserID    string          `json:"user_id"`
	ProjectID string          `json:"project_id"`
	Grants    []GrantRef      `json:"grants"`
	Request   json.RawMessage `json:"request"`
	Response  json.RawMessage `json:"response"`
	Duration  time.Duration   `json:"duration"`
}

// Response is the envelope Zitadel Actions v2 expects back.
type Response struct {
	AppendClaims    []Claim  `json:"append_claims"`
	AppendLogClaims []string `json:"append_log_claims,omitempty"`
}

// Deadline bounds a whole call (assessment §4e, P7 kept as a belt). Zitadel
// gives a call 3s including connect and TLS; the answer is memory plus one
// Redis read bounded at FactsTimeout, so this is never reached unless
// something is badly wrong.
const Deadline = 250 * time.Millisecond

// BodyCap is the largest request either door reads.
const BodyCap = 1 << 20

// Handler is the whole route: counted, then verified, then answered.
func (d *Door) Handler() http.HandlerFunc { return d.Delivery(d.Signature(d.Inject)) }

// Delivery wraps the WHOLE route, signature check included, so every call is
// counted exactly once whatever stops it: a refused signature, an
// undecodable body, a degraded answer, or a panic. A panic is answered 500
// here and counted as one; Zitadel issues the token without Syndra's claims
// either way, and the count is what makes it loud.
func (d *Door) Delivery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		call := &claimtel.Call{Outcome: claimtel.BadRequest, At: time.Now()}
		defer func() {
			if rec := recover(); rec != nil {
				call.Outcome = claimtel.Panic
				log.Printf("[PANIC] data plane %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong handling that request.", nil)
			}
			call.Elapsed = time.Since(call.At)
			if d.Record != nil {
				d.Record(*call)
			}
		}()
		next(w, r.WithContext(claimtel.With(r.Context(), call)))
	}
}

// Signature verifies the call against Keys, then the next handler reads the
// same body again.
func (d *Door) Signature(next http.HandlerFunc) http.HandlerFunc {
	return SignatureCheck("ZITADEL_ACTION_SIGNING_KEY", d.Keys, d.OnMismatch, d.DevMode, next)
}

// SignatureCheck verifies against each of keys() in turn. With no key at all
// it passes through only in dev mode, and refuses otherwise (May 2026 audit
// C1). onMismatch, when set, hears every call whose signature matched none.
func SignatureCheck(envName string, keys func() []SlotKey, onMismatch func(), devMode func() bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ks []SlotKey
		if keys != nil {
			ks = keys()
		}
		if len(ks) == 0 {
			if devMode != nil && devMode() {
				log.Printf("[ACTION] %s unset and ZITADEL_DOMAIN unset — signature verification disabled (dev mode)", envName)
				next(w, r)
				return
			}
			// No key can match, so it counts as a mismatch.
			claimtel.From(r.Context()).Set(claimtel.SigMismatch)
			log.Printf("[ACTION] %s unset while ZITADEL_DOMAIN set — refusing request (misconfiguration)", envName)
			writeError(w, http.StatusServiceUnavailable, "MISCONFIGURED", "Signing key not configured for this endpoint", nil)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			claimtel.From(r.Context()).Set(claimtel.BadRequest)
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body", nil)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		slot, err := VerifySignature(body, r.Header.Get(SignatureHeader), ks, SignatureTolerance, time.Now())
		if err != nil {
			o := SignatureOutcome(err)
			claimtel.From(r.Context()).Set(o)
			if o == claimtel.SigMismatch && onMismatch != nil {
				onMismatch()
			}
			log.Printf("[ACTION] Signature verification failed: %v", err)
			writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Invalid or missing Zitadel signature", nil)
			return
		}
		if call := claimtel.From(r.Context()); call != nil {
			call.KeySlot = slot
		}
		next(w, r)
	}
}

// Inject answers one Actions v2 call from the snapshot: config from memory,
// facts from one bounded Redis read, never Postgres or Zitadel.
//
// One project per token. Zitadel lists in user_grants only the token's role
// audience: the client's own project, plus any project the app asked for with
// an `urn:zitadel:iam:org:project:id:{id}:aud` scope. Syndra delivers claims
// for exactly one project and refuses cross-project delivery outright.
// ponytail: refuse rather than pick the client's own project; resolve
// application.client_id to its project if an app ever needs extra audiences.
//
// Debug mode is asked once, after the answer is written, so the latency path
// is untouched.
func (d *Door) Inject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	call := claimtel.From(r.Context())
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported", nil)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, BodyCap))
	if err != nil {
		call.Set(claimtel.BadRequest)
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req, fault := Decode(body)
	if fault == claimtel.BadRequest {
		call.Set(claimtel.BadRequest)
		writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid JSON payload", map[string]string{"body": "not a JSON object"})
		return
	}
	if call != nil {
		call.Function = req.Function
		if call.Function == "" {
			call.Function = claimtel.None
		}
		call.ClientID, call.UserID, call.Body = req.Application.ClientID, req.User.ID, body
		call.Probe = req.User.ID == CanaryUserID
	}
	if fault == claimtel.ContractFault {
		call.Set(claimtel.ContractFault)
		if strings.TrimSpace(req.User.ID) == "" {
			writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", "user.id is required", map[string]string{"user.id": "required"})
			return
		}
		log.Printf("[DATA PLANE] Contract fault: the payload's user_grants do not read as Zitadel's (client=%q function=%s); answering no claims", req.Application.ClientID, req.Function)
		writeJSON(w, http.StatusOK, Response{AppendClaims: []Claim{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), Deadline)
	defer cancel()
	ans := d.Answer(ctx, req)
	if call != nil {
		call.Outcome, call.ProjectID, call.ClientKnown, call.Gen = ans.Outcome, ans.ProjectID, ans.ClientKnown, ans.Gen
	}
	resp := Response{AppendClaims: ans.Claims}
	writeJSON(w, http.StatusOK, resp)

	if d.Debug != nil && d.Debug.Wants(req.Application.ClientID, req.User.ID) {
		answer, _ := json.Marshal(resp)
		d.Debug.Capture(DebugCapture{
			At: start, Function: req.Function, ClientID: req.Application.ClientID, UserID: req.User.ID,
			ProjectID: ans.ProjectID, Grants: req.UserGrants, Request: body, Response: answer, Duration: time.Since(start),
		})
	}
}

// Answer is the token's answer, the one project it is for, and its outcome.
type Answer struct {
	Claims      []Claim
	ProjectID   string // "" when the grants name none, or several and it is refused
	Outcome     claimtel.Outcome
	ClientKnown bool
	Gen         int64
}

// Answer answers a decoded request.
func (d *Door) Answer(ctx context.Context, req Request) Answer {
	projects := req.Projects()
	switch {
	case len(projects) == 0:
		// A first sign-in races onboarding (SC4): the token names no project
		// yet. With the welcome switch on, a brand-new member gets the welcome
		// roles for the app's own project.
		if res, pid, ok := d.Plane().Welcome(ctx, req.User, req.Application.ClientID, req.Function); ok {
			return Answer{Claims: res.Claims, ProjectID: pid, Outcome: res.Outcome, ClientKnown: res.ClientKnown, Gen: res.Gen}
		}
		return Answer{Claims: []Claim{}, Outcome: claimtel.NoGrants}
	case len(projects) > 1:
		log.Printf("[DATA PLANE] Refusing cross-project claims for user=%s client=%q function=%s: the token spans projects %v. "+
			"Syndra delivers one project per token; drop the extra :aud scopes, or give the role in the app's own project with a rule.",
			req.User.ID, req.Application.ClientID, req.Function, projects)
		return Answer{Claims: []Claim{}, Outcome: claimtel.RefusedCrossProject}
	}
	res := d.Plane().Answer(ctx, req.User.ID, projects[0], req.Application.ClientID, req.Function)
	if res.Miss && d.OnMiss != nil {
		// The first sign that a person was never compiled: answer degraded
		// now, recompile off the hot path so the next token finds them (F-7).
		d.OnMiss(req.User.ID)
	}
	return Answer{Claims: res.Claims, ProjectID: projects[0], Outcome: res.Outcome, ClientKnown: res.ClientKnown, Gen: res.Gen}
}

// Router serves only the data plane: the claims target and its health. No
// CORS, no cancel gate, no admin route shares its mux (S18), and a panic
// anywhere in it is a 500 that leaves the process serving.
func Router(inject, health http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/action/inject", inject)
	mux.HandleFunc("GET /healthz/claims", health)
	return PanicGuard(MaxBody(mux))
}

// Addr is the claims process's listen address: SYNDRA_CLAIMS_ADDR, else :8090.
func Addr() string {
	if a := os.Getenv("SYNDRA_CLAIMS_ADDR"); a != "" {
		return a
	}
	return ":8090"
}

// NewServer is the data plane's own http.Server (P2). Zitadel's whole budget
// is 3s; a connection that has not delivered its request in 2s, or cannot
// take its answer in 2s, is closed rather than held.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

// PanicGuard answers a panic 500 and keeps serving. It logs the method, path,
// value and stack, never the body or headers.
func PanicGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			log.Printf("[PANIC] %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong handling that request.", nil)
		}()
		next.ServeHTTP(w, r)
	})
}

// MaxBody caps a request body at BodyCap.
func MaxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, BodyCap)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Failed to encode json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string, details map[string]string) {
	writeJSON(w, status, struct {
		Error   string            `json:"error"`
		Message string            `json:"message"`
		Details map[string]string `json:"details,omitempty"`
	}{code, msg, details})
}
