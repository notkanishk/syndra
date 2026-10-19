package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"syndra/internal/db"
	"syndra/internal/models"
)

// cancelAckHeader carries the queued change ids an operator was shown and
// agreed to cancel. The UI proxy forwards it; nothing else sets it.
const cancelAckHeader = "X-Syndra-Cancel-Ack"

var queuedChangeID = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)

// WouldCancelResponse is the refusal an operator's client turns into a
// confirmation: what would be cancelled, and that nothing changed.
type WouldCancelResponse struct {
	Error   string              `json:"error"`
	Message string              `json:"message"`
	Cancels []WouldCancelChange `json:"cancels"`
}

// WouldCancelChange is a queued change the request would cancel. New is set
// when the retry had acknowledged others and this one was not among them: it
// was queued since the operator last looked. InitiatedByName is who started
// it, when the directory knows them.
type WouldCancelChange struct {
	models.QueuedChange
	New             bool   `json:"new,omitempty"`
	InitiatedByName string `json:"initiated_by_name,omitempty"`
}

// withCancelGate gates every mutating request (see db.CancelGate). If the one
// place cancellations happen refused, the response is that refusal, whatever
// the handler then wrote: its transaction rolled back, so nothing changed,
// and a handler that reported success or a generic failure would be wrong.
func withCancelGate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next(w, r)
			return
		}
		// Only an operator is asked: a member's own request (the welcome bundle
		// on first sign-in) has nobody to ask, and is audited like system work.
		acked := parseCancelAck(r.Header.Get(cancelAckHeader))
		ctx, gate := db.GuardCancellations(r.Context(), acked, isOperatorCtx)
		next(&gateWriter{ResponseWriter: w, gate: gate, ctx: ctx, acked: acked}, r.WithContext(ctx))
	}
}

func parseCancelAck(header string) []string {
	var ids []string
	for _, part := range strings.Split(header, ",") {
		if id := strings.TrimSpace(part); queuedChangeID.MatchString(id) && len(ids) < 200 {
			ids = append(ids, id)
		}
	}
	return ids
}

// gateWriter replaces the handler's response with the refusal, at the first
// byte it would have written.
type gateWriter struct {
	http.ResponseWriter
	gate    *db.CancelGate
	ctx     context.Context
	acked   []string
	decided bool
	swallow bool
}

func (g *gateWriter) decide() {
	if g.decided {
		return
	}
	g.decided = true
	refused := g.gate.Refused()
	if len(refused) == 0 {
		return
	}
	g.swallow = true
	noun := "change that is"
	if len(refused) != 1 {
		noun = "changes that are"
	}
	jsonResponse(g.ResponseWriter, http.StatusConflict, WouldCancelResponse{
		Error:   "WOULD_CANCEL_QUEUED",
		Message: fmt.Sprintf("This would cancel %d %s waiting for you to send. Nothing was changed.", len(refused), noun),
		Cancels: g.describe(refused),
	})
}

// describe marks what is new since the acknowledgement and names who started
// each change. Names come from the directory's cached list; if it can't answer
// the refusal is sent without them rather than not at all.
func (g *gateWriter) describe(refused []models.QueuedChange) []WouldCancelChange {
	seen := make(map[string]bool, len(g.acked))
	for _, id := range g.acked {
		seen[id] = true
	}
	var names map[string]string
	if users, err := directoryUsers(g.ctx); err == nil {
		names = make(map[string]string, len(users))
		for _, u := range users {
			names[u.ID] = u.Name
		}
	}
	out := make([]WouldCancelChange, len(refused))
	for i, c := range refused {
		out[i] = WouldCancelChange{
			QueuedChange:    c,
			New:             len(seen) > 0 && !seen[c.ID],
			InitiatedByName: names[c.InitiatedBy],
		}
	}
	return out
}

func (g *gateWriter) WriteHeader(status int) {
	g.decide()
	if !g.swallow {
		g.ResponseWriter.WriteHeader(status)
	}
}

func (g *gateWriter) Write(b []byte) (int, error) {
	g.decide()
	if g.swallow {
		return len(b), nil
	}
	return g.ResponseWriter.Write(b)
}
