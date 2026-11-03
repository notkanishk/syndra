// Package claimcheck is the read-only reconciler of Zitadel's claims
// configuration (claims-delivery-integrity S4, assessment §4c): checks R1 to
// R9 compare what Zitadel and Syndra hold with what claim delivery needs,
// before any sign-in fails. Evaluate is pure: the caller reads Zitadel, the
// directory and the snapshot, and hands the answers in, so every check is a
// table test over recorded JSON.
//
// Rows carry values, not sentences: the UI words them (the copy guide is
// enforced there).
package claimcheck

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
	"syndra/internal/zitadel"
)

// The manifest (zitadel/actions/targets.json): what register.sh applies.
// TestManifestMatchesTargetsJSON keeps these equal to the file.
const (
	ClaimTarget = "syndra-claim-injector"
	EventTarget = "syndra-event-listener"
	ClaimPath   = "/api/action/inject"
	EventPath   = "/api/webhooks/zitadel"
	// ClaimTimeout is the claims target's timeout.
	ClaimTimeout = 3 * time.Second
)

// ClaimFunctions are the functions bound to the claims target.
var ClaimFunctions = []string{"preaccesstoken", "preuserinfo"}

// ManifestEvents are the events bound to the event listener.
var ManifestEvents = []string{
	"user.human.added", "user.human.selfregistered", "user.deactivated", "user.locked",
	"user.grant.added", "user.grant.changed", "user.grant.removed",
	"oidc_session.added", "user.token.added",
}

// Row states.
const (
	Pass      = "pass"
	Drift     = "drift"      // a CONFIG_DRIFT finding
	Info      = "info"       // worth knowing, not a finding
	Cannot    = "cannot"     // the read failed: a CANNOT_CHECK finding
	NotNeeded = "not_needed" // nothing to check on this Zitadel
)

// Row is one check of one thing. Subject is the finding's subject: the check
// and what it looked at ("R1:endpoint", "R5:<app id>").
type Row struct {
	Check    string         `json:"check"`
	Subject  string         `json:"subject"`
	State    string         `json:"state"`
	Expected any            `json:"expected,omitempty"`
	Found    any            `json:"found,omitempty"`
	Facts    map[string]any `json:"facts,omitempty"`
}

// App is one live app as R5 and R6 need it.
type App struct {
	ID, Name, ClientID, ProjectID string
	OIDC                          bool
	Settings                      *zitadel.TokenSettings // nil with SettingsErr
	SettingsErr                   error
}

// Inputs is everything one pass read. A non-nil *Err means that read failed:
// its checks are CANNOT_CHECK, never a guess.
type Inputs struct {
	ExternalURL string

	Targets       []zitadel.ActionTarget
	TargetsErr    error
	Executions    []zitadel.ActionExecution
	ExecutionsErr error
	V1            []zitadel.TokenAction
	V1Err         error
	Apps          []App
	AppsErr       error

	// Snapshot is the data plane's config: every published project's shapes.
	Snapshot *claimserve.Config
	// ProjectRoles are each project's role keys. Any project whose roles
	// could not be read is RolesErr, so R7 never settles what it did not see.
	ProjectRoles map[string][]string
	RolesErr     error

	// HeldKeys is KeyCheck of every key the data plane verifies with now
	// (the keyring and the env key); EnvKey is KeyCheck of the env key alone.
	HeldKeys []string
	EnvKey   string
	// ResolveErr is the external URL's DNS lookup.
	ResolveErr error
}

// KeyCheck is a signing key's fingerprint for comparing keys without holding
// both: HMAC(key, "syndra-key-check"). Never log the key itself; "" for "".
func KeyCheck(key string) string {
	if key == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("syndra-key-check"))
	return hex.EncodeToString(mac.Sum(nil))
}

// Report is one pass.
type Report struct {
	Rows []Row `json:"rows"`
	// JWT is each OIDC client's access-token type as R5 read it: what the
	// correlator expects a sign-in to call.
	JWT map[string]bool `json:"-"`
}

// Ran is the checks that read what they needed (no Cannot row).
func (r Report) Ran() []string {
	cannot := map[string]bool{}
	for _, row := range r.Rows {
		if row.State == Cannot {
			cannot[row.Check] = true
		}
	}
	var out []string
	for _, c := range []string{"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9"} {
		if !cannot[c] {
			out = append(out, c)
		}
	}
	return out
}

// Reason is what a failed read says: the HTTP status Zitadel answered, or
// the error, so "no permission" reads differently from "not reachable".
func Reason(err error) map[string]any {
	var se *zitadel.StatusError
	if errors.As(err, &se) {
		return map[string]any{"status": se.Code}
	}
	return map[string]any{"error": err.Error()}
}

func cannot(check string, err error) Row {
	return Row{Check: check, Subject: check, State: Cannot, Facts: Reason(err)}
}

func cmp(check, field string, want, got any, ok bool) Row {
	r := Row{Check: check, Subject: check + ":" + field, State: Pass, Expected: want, Found: got}
	if !ok {
		r.State = Drift
	}
	return r
}

// Evaluate runs every check.
func Evaluate(in Inputs) Report {
	rep := Report{JWT: map[string]bool{}}
	add := func(rows ...Row) { rep.Rows = append(rep.Rows, rows...) }
	base := strings.TrimRight(in.ExternalURL, "/")
	if in.Snapshot == nil {
		// Before the first load: R5 to R7 have nothing to compare with.
		in.Snapshot = &claimserve.Config{}
		noSnap := errors.New("the claims snapshot is not loaded yet")
		in.AppsErr = errors.Join(in.AppsErr, noSnap)
		if in.RolesErr == nil {
			in.RolesErr = noSnap
		}
	}

	byName := map[string]zitadel.ActionTarget{}
	names := map[string]string{}
	for _, t := range in.Targets {
		byName[t.Name] = t
		names[t.ID] = t.Name
	}

	// R1 · the claims target.
	if in.TargetsErr != nil {
		add(cannot("R1", in.TargetsErr), cannot("R2", in.TargetsErr), cannot("R3", in.TargetsErr))
	} else if t, ok := byName[ClaimTarget]; !ok {
		add(cmp("R1", "present", ClaimTarget, nil, false))
	} else {
		facts := map[string]any{"target_id": t.ID, "changed": t.ChangeDate}
		r := cmp("R1", "present", ClaimTarget, ClaimTarget, true)
		r.Facts = facts
		add(r)
		add(cmp("R1", "endpoint", base+ClaimPath, t.Endpoint, t.Endpoint == base+ClaimPath))
		add(cmp("R1", "type", "restCall", t.Type(), t.Type() == "restCall"))
		interrupt := t.RestCall != nil && t.RestCall.InterruptOnError
		add(cmp("R1", "interrupt_on_error", false, interrupt, !interrupt))
		d, err := time.ParseDuration(t.Timeout)
		add(cmp("R1", "timeout", ClaimTimeout.String(), t.Timeout, err == nil && d == ClaimTimeout))
		// Unspecified is JSON (the field is newer than v4.8).
		pt := t.PayloadType
		add(cmp("R1", "payload_type", "PAYLOAD_TYPE_JSON", pt, pt == "" || pt == "PAYLOAD_TYPE_JSON" || pt == "PAYLOAD_TYPE_UNSPECIFIED"))
		// The key Zitadel signs with must be one the data plane verifies with.
		zk := KeyCheck(t.SigningKey)
		held := zk != "" && contains(in.HeldKeys, zk)
		add(cmp("R1", "signing_key", "held", map[bool]string{true: "held", false: "not_held"}[held], held))
		if in.EnvKey != "" && zk != "" && in.EnvKey != zk {
			// The keyring follows Zitadel; the env copy is only the fallback.
			add(Row{Check: "R9", Subject: "R9:env_key", State: Info, Expected: "matches", Found: "older"})
		}
	}

	// R2 · the functions call exactly the claims target.
	if in.ExecutionsErr != nil && in.TargetsErr == nil {
		add(cannot("R2", in.ExecutionsErr), cannot("R3", in.ExecutionsErr))
	} else if in.TargetsErr == nil {
		for _, fn := range ClaimFunctions {
			got := []string{}
			for _, e := range in.Executions {
				if e.FunctionName() == fn {
					got = append(got, targetNames(e.TargetIDs(), names)...)
				}
			}
			r := cmp("R2", fn, []string{ClaimTarget}, got, len(got) == 1 && got[0] == ClaimTarget)
			r.Facts = map[string]any{"function": fn}
			add(r)
		}
	}

	// R3 · the event listener and every event it should hear.
	if in.TargetsErr == nil && in.ExecutionsErr == nil {
		t, ok := byName[EventTarget]
		add(cmp("R3", "target", base+EventPath, t.Endpoint, ok && t.Endpoint == base+EventPath && t.Type() == "restAsync"))
		for _, ev := range ManifestEvents {
			bound := false
			for _, e := range in.Executions {
				if e.EventName() == ev && contains(targetNames(e.TargetIDs(), names), EventTarget) {
					bound = true
				}
			}
			r := cmp("R3", ev, EventTarget, map[bool]string{true: EventTarget, false: ""}[bound], bound)
			r.Facts = map[string]any{"event": ev}
			add(r)
		}
	}

	// R4 · Actions v1, while it lives: the keys its scripts set, and which of
	// them a shape also sends (v1 runs first, so Syndra's value is dropped).
	// Informational: the owner retires v1 himself.
	if in.V1Err != nil {
		add(cannot("R4", in.V1Err))
	} else {
		keys := v1Keys(in.V1)
		shapeKeys := map[string]bool{}
		for _, p := range in.Snapshot.Projects {
			for _, k := range topKeys(p.Shapes.Default) {
				shapeKeys[k] = true
			}
			for _, s := range p.Shapes.ByClient {
				for _, k := range topKeys(s.Profile) {
					shapeKeys[k] = true
				}
			}
		}
		collide := []string{}
		for _, k := range keys {
			if shapeKeys[k] {
				collide = append(collide, k)
			}
		}
		state := Pass
		if len(keys) > 0 {
			state = Info
		}
		add(Row{Check: "R4", Subject: "R4:v1", State: state, Found: keys, Facts: map[string]any{"collide": collide, "keys": len(keys)}})
	}

	// R5 · each app's claims reach where its shape lands. R6 · every shape
	// names a live app, and every app's project is published.
	if in.AppsErr != nil {
		add(cannot("R5", in.AppsErr), cannot("R6", in.AppsErr))
	} else {
		var settingsErr error
		live := map[string]bool{}
		for _, a := range in.Apps {
			if a.ClientID != "" {
				live[a.ClientID] = true
			}
			if !a.OIDC || a.ClientID == "" {
				continue
			}
			if a.SettingsErr != nil {
				settingsErr = a.SettingsErr
				continue
			}
			if a.Settings == nil {
				continue
			}
			rep.JWT[a.ClientID] = a.Settings.AccessTokenType == "JWT"
			proj, ok := in.Snapshot.Projects[a.ProjectID]
			if !ok {
				continue // R6 says so
			}
			shape, _ := proj.Shapes.For(a.ClientID)
			if len(shape.Profile.Claims) == 0 {
				continue // sign-in only: nothing to land
			}
			reached := claims.ReachedBy(shape.Profile.Lands, a.Settings.AccessTokenType == "JWT", a.Settings.IDTokenUserinfoAssertion)
			tpl, _ := claims.FindTemplate(shape.TemplateID)
			ok = reached[claims.ReadsAccessToken] || reached[claims.ReadsIDToken] || reached[claims.ReadsUserinfo]
			if tpl.Reads != "" {
				ok = reached[tpl.Reads]
			} else if shape.Profile.Lands == claims.LandsAccessToken {
				ok = reached[claims.ReadsAccessToken]
			}
			add(Row{Check: "R5", Subject: "R5:" + a.ID, State: map[bool]string{true: Pass, false: Drift}[ok],
				Expected: shape.Profile.Lands, Found: map[string]any{"access_token_type": a.Settings.AccessTokenType, "id_token_userinfo_assertion": a.Settings.IDTokenUserinfoAssertion},
				Facts: map[string]any{"app_id": a.ID, "project_id": a.ProjectID, "reads": tpl.Reads}})
		}
		if settingsErr != nil {
			add(cannot("R5", settingsErr))
		}
		for pid, p := range in.Snapshot.Projects {
			for client, s := range p.Shapes.ByClient {
				if !live[client] {
					add(Row{Check: "R6", Subject: "R6:shape:" + client, State: Drift, Expected: "live", Found: "gone",
						Facts: map[string]any{"client_id": client, "app_id": s.ApplicationID, "project_id": pid}})
				}
			}
		}
		for _, a := range in.Apps {
			if a.ClientID == "" || a.ProjectID == "" {
				continue
			}
			_, ok := in.Snapshot.Projects[a.ProjectID]
			r := cmp("R6", "app:"+a.ID, "published", map[bool]string{true: "published", false: "missing"}[ok], ok)
			r.Facts = map[string]any{"app_id": a.ID, "project_id": a.ProjectID}
			add(r)
		}
	}

	// R7 · the roles each shape names exist in its project.
	if in.RolesErr != nil {
		add(cannot("R7", in.RolesErr))
	} else {
		for pid, p := range in.Snapshot.Projects {
			have, ok := in.ProjectRoles[pid]
			if !ok {
				continue
			}
			set := map[string]bool{}
			for _, k := range have {
				set[k] = true
			}
			missing := map[string][]string{} // role -> the apps (or "" for the default) naming it
			note := func(prof claims.Profile, appID string) {
				for _, k := range roleKeys(prof) {
					if !set[k] {
						missing[k] = append(missing[k], appID)
					}
				}
			}
			note(p.Shapes.Default, "")
			for _, s := range p.Shapes.ByClient {
				note(s.Profile, s.ApplicationID)
			}
			if len(missing) == 0 {
				add(Row{Check: "R7", Subject: "R7:" + pid, State: Pass, Facts: map[string]any{"project_id": pid}})
			}
			for role, apps := range missing {
				sort.Strings(apps)
				add(Row{Check: "R7", Subject: "R7:" + pid + ":" + role, State: Drift, Expected: "in project", Found: "gone",
					Facts: map[string]any{"project_id": pid, "role": role, "app_ids": apps}})
			}
		}
	}

	// R8 · instance features: Zitadel v3 had a switch for v2 executions; v4
	// has none (assessment Z14). Shown so the list is the same nine checks.
	add(Row{Check: "R8", Subject: "R8", State: NotNeeded})

	// R9 · Syndra's own settings: the snapshot and the warm are already
	// /healthz/claims reasons, so only the environment here.
	add(cmp("R9", "key_present", true, in.EnvKey != "", in.EnvKey != ""))
	https := strings.HasPrefix(in.ExternalURL, "https://")
	add(cmp("R9", "url_https", true, https, https))
	r := cmp("R9", "url_resolves", true, in.ResolveErr == nil, in.ResolveErr == nil)
	if in.ResolveErr != nil {
		r.Facts = map[string]any{"error": in.ResolveErr.Error()}
	}
	add(r)

	// R1…R9 sort as strings; within a check, by subject, so a pass is stable.
	sort.SliceStable(rep.Rows, func(i, j int) bool {
		a, b := rep.Rows[i], rep.Rows[j]
		return a.Check < b.Check || (a.Check == b.Check && a.State == Cannot && b.State != Cannot)
	})
	return rep
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func targetNames(ids []string, names map[string]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if n, ok := names[id]; ok {
			out = append(out, n)
		} else {
			out = append(out, id)
		}
	}
	return out
}

var setClaim = regexp.MustCompile(`setClaim\(\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)

// v1Keys is every top-level claim key an active v1 script sets, sorted.
func v1Keys(actions []zitadel.TokenAction) []string {
	seen := map[string]bool{}
	for _, a := range actions {
		if a.State != "" && a.State != "ACTION_STATE_ACTIVE" {
			continue
		}
		for _, m := range setClaim.FindAllStringSubmatch(a.Script, -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func topKeys(p claims.Profile) []string {
	var out []string
	for _, c := range p.Claims {
		if len(c.Path) > 0 {
			out = append(out, c.Path[0])
		}
	}
	return out
}

// roleKeys is every role a shape names: its role list and its if-role rows.
func roleKeys(p claims.Profile) []string {
	var out []string
	for _, m := range p.Roles {
		out = append(out, m.Key)
	}
	for _, c := range p.Claims {
		out = append(out, c.Roles...)
	}
	return out
}

// Observations turns a report into findings: a drift row fails its subject,
// a pass row is a clean look at it, and a check that could not read is one
// CANNOT_CHECK subject.
func (r Report) Observations(at time.Time) (drift, cannotCheck []claimtel.Observation) {
	for _, row := range r.Rows {
		sample := map[string]any{"expected": row.Expected, "found": row.Found}
		for k, v := range row.Facts {
			sample[k] = v
		}
		switch row.State {
		case Drift:
			drift = append(drift, claimtel.Observation{Kind: claimtel.KindConfigDrift, Subject: row.Subject, Fail: true, At: at, Sample: sample})
		case Pass:
			drift = append(drift, claimtel.Observation{Kind: claimtel.KindConfigDrift, Subject: row.Subject, At: at})
		case Cannot:
			cannotCheck = append(cannotCheck, claimtel.Observation{Kind: claimtel.KindCannotCheck, Subject: row.Check, Fail: true, At: at, Sample: sample})
		}
	}
	for _, c := range r.Ran() {
		cannotCheck = append(cannotCheck, claimtel.Observation{Kind: claimtel.KindCannotCheck, Subject: c, At: at})
	}
	return drift, cannotCheck
}

// Seen is every subject this report looked at, for settling the ones it no
// longer finds.
func (r Report) Seen() []string {
	out := make([]string, 0, len(r.Rows))
	for _, row := range r.Rows {
		out = append(out, row.Subject)
	}
	return out
}
