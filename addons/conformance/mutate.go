package conformance

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// mutate runs every check that writes, against one throwaway subject. Its
// order matters: withdraw-only and the refusals run before the account
// exists, and whatever happens after the first write, finish runs last.
func (k *kit) mutate() error {
	pre := k.planOne(k.at("plan"), k.full())
	if k.ctx.Err() != nil {
		k.halted = "not reached: the run was interrupted"
		return ErrInterrupted
	}
	if pre == nil {
		k.halted = "not reached: the kit's subject could not be planned (plan)"
		return ErrReadsFailed
	}
	if exists(pre) {
		k.halted = "not reached: the kit's subject already exists, and the kit never touches what it did not create"
		return ErrSubjectExists
	}
	// Every account there before the first write. Without it the kit could not
	// tell its own account from someone else's, so it writes nothing: not when
	// the read fails, and not when it is truncated and so proves nothing.
	before := &check{}
	doc := k.ok(before, k.currentSubjects(), "subjects.json")
	switch {
	case k.ctx.Err() != nil:
		k.halted = "not reached: the run was interrupted"
		return ErrInterrupted
	case len(before.fails) > 0:
		k.halted = "not reached: the accounts before the first write could not be read: " + strings.Join(before.fails, "; ")
		return ErrReadsFailed
	case doc["truncated"] != false:
		k.halted = "not reached: the accounts before the first write were read truncated, so the kit cannot prove which accounts existed"
		return ErrReadsFailed
	case doc["current"] != true:
		k.halted = "not reached: the accounts before the first write were not a current read (an answer from a mirror), so the kit cannot prove which accounts exist"
		return ErrReadsFailed
	}
	k.before = idsOf(doc)
	defer k.finish()

	var callID string
	var created *exchange
	// Each step names the checks it judges, so a run stopped before or during
	// a step marks them not judged rather than letting half a check pass.
	type step struct {
		name   string
		checks []string
		run    func()
	}
	steps := []step{
		{"withdraw-only", []string{"withdraw-only.no-account", "plan-equals-apply"}, func() { k.withdrawWithoutAccount(k.at("withdraw-only.no-account")) }},
		{"stale", []string{"replay.refusal"}, func() { k.staleRefusal(k.at("replay.refusal")) }},
		{"fingerprint", []string{"fingerprint"}, func() { k.fingerprint(k.at("fingerprint")) }},
		{"strict", []string{"strict-requests"}, func() {
			k.strict(k.at("strict-requests"), "/apply", http.MethodPost,
				k.applyBody(k.callID(), "conformance-wrong-fingerprint", map[string]any{k.m.account: false}), nil)
		}},
		{"settings", []string{"settings"}, func() { k.settingsWrite(k.at("settings")) }},
		{"create", []string{"plan-equals-apply", "create-is-new", "health"}, func() {
			k.at("replay.29d").skip("needs a clock the kit can move; an add-on's clock cannot be moved from outside")
			logBefore, logOK := k.logRecords(k.at("health"))
			callID = k.callID()
			created, _ = k.converge(k.at("plan-equals-apply"), callID, k.full(), true)
			if k.created.id != "" {
				k.advanced(k.at("health"), logBefore, logOK, "the apply that created the kit's account")
			}
		}},
	}
	withAccount := []step{
		{"read-back", []string{"read-back"}, func() { k.readBack(k.at("read-back"), created.doc()) }},
		{"limits", []string{"limit-combine"}, func() { k.limits(k.at("limit-combine")) }},
		{"replay", []string{"replay"}, func() { k.replay(k.at("replay"), callID, created) }},
		{"absent", []string{"absent-unmanaged", "plan-equals-apply"}, func() { k.absent(k.at("absent-unmanaged")) }},
		{"limit-empty", []string{"limit-empty", "plan-equals-apply"}, func() { k.limitEmpty(k.at("limit-empty")) }},
		{"operations", []string{"operations", "operations.secret", "strict-requests", "health"}, func() { k.operations(k.at("operations")) }},
		{"calls", []string{"calls.recorded", "operations.physical", "health"}, func() { k.callsWrite() }},
		{"reads", []string{"operations.reads", "operations.result"}, func() {
			k.readOperations(k.at("operations.reads"), callID)
			k.results(k.at("operations.result"), true)
		}},
		{"stats", []string{"stats.points", "stats.totals", "stats.since", "stats.gaps", "stats.unbound", "stats.no-people", "stats.no-people.ids"}, func() { k.statsAfterWrites() }},
		{"lifecycle", []string{"lifecycle", "replay.refusal", "strict-requests"}, func() { k.lifecycle(k.at("lifecycle"), k.at("replay.refusal")) }},
		{"withdraw", []string{"withdraw-only.existing", "channel-follows-account", "plan-equals-apply"}, func() {
			k.withdrawExisting(k.at("withdraw-only.existing"), k.at("channel-follows-account"))
		}},
	}
	all := append(steps, withAccount...)
	stop := func(from int, why string) {
		for _, s := range all[from:] {
			k.cutShort(why, s.checks...)
		}
	}
	for i, s := range all {
		if k.ctx.Err() != nil {
			k.halted = "not reached: the run was interrupted"
			return ErrInterrupted
		}
		if i == len(steps) && k.created.id == "" {
			k.at("plan-equals-apply").fail("the apply that creates the kit's account answered no account the kit could own")
			stop(i, "needs the kit's account, and the apply that creates it answered none it could own (see plan-equals-apply, create-is-new)")
			return nil
		}
		k.step = s.name
		s.run()
		if k.frozen() { // this step is cut short too
			stop(i, "the kit stopped writing to its subject during "+s.name+" (see bound-to-created)")
			return nil
		}
	}
	return nil
}

// cutShort marks checks whose remaining probes will not run as not judged
// (unless they already failed), so none passes on half of what it asserts.
func (k *kit) cutShort(why string, ids ...string) {
	for _, id := range ids {
		if c := k.at(id); c.notJudged == "" {
			c.notJudged = "not judged: the run stopped before this check's remaining probes; " + why
		}
	}
}

// finish ends every -mutate run that wrote, interrupted or not: it puts
// lifecycle back to active, then turns off the kit's own account, if it owns
// one and nothing foreign was seen.
func (k *kit) finish() {
	k.cleaning, k.step = true, "end"
	k.restoreActive()
	k.disableOwn()
	// A probe a broken add-on accepted may have created what nobody asked for.
	// The last line says so; the kit touches nothing that is not its own.
	for _, id := range k.accountIDs(&check{}) {
		if !slices.Contains(k.before, id) && !k.owns(id) && !slices.Contains(k.foreign, id) {
			k.appeared = append(k.appeared, id)
		}
	}
}

// disableOwn re-plans the subject once the add-on has nothing in flight. The
// plan goes through owner() like every other reply: an id the kit does not own
// (including one first seen here) is foreign, is reported, and freezes writes,
// so the apply below is only ever sent to the account the kit created. "No
// account" is believed only when every write was answered.
func (k *kit) disableOwn() {
	defer func() {
		// Without the end-of-run plan, whether the subject stayed bound to the
		// kit's account to the end is not known.
		if c := k.at("bound-to-created"); !k.finalRead && c.notJudged == "" {
			c.notJudged = "not judged: the end-of-run plan for the subject was never read (" + k.unknown + ")"
		}
	}()
	if !k.drained() {
		k.unknown = "the add-on never reported in_flight 0, so a write may still land"
		return
	}
	desired := map[string]any{k.m.account: false}
	var plan map[string]any
	if k.created.id == "" || k.frozen() {
		plan = k.planOne(&check{}, desired) // read only: owner() still judges what it names
	} else {
		plan, _ = k.toOwned(&check{}, desired, func(plan map[string]any) *request {
			if str(plan, "effect") != "apply" {
				return nil // already off
			}
			return &request{method: http.MethodPost, target: "/apply", schema: "apply_request.json",
				body: k.applyBody(k.callID(), str(plan, "fingerprint"), desired)}
		})
	}
	k.finalRead = plan != nil
	switch {
	case plan == nil:
		k.unknown = "the final plan for the subject could not be read"
	case k.frozen() && k.created.id != "":
		k.unknown = "the kit stopped writing to its subject, so its account was not turned off at the end"
		if k.stopReason != "" {
			k.unknown = k.stopReason
		}
	case k.frozen():
		// Nothing of the kit's own; the last line says what may have reached what.
	case k.created.id != "": // toOwned sent the disable, or found it already off
		k.created.off = k.state(&check{})[k.m.account] == false
	case k.unanswered:
		k.unknown = "a write went unanswered, and the plan's word that there is no account is not proof"
	case str(plan, "effect") == "no_change" && plan["standing"] == nil:
		k.created.none = true
	default:
		k.unknown = "the final plan neither names an account nor says there is none"
	}
}

// drained waits, about a minute at most, for health to say in_flight 0: a
// write the kit stopped waiting for may still be running in the add-on, and
// the final plan must not be read before it lands.
func (k *kit) drained() bool {
	wait := 100 * time.Millisecond
	for deadline := time.Now().Add(k.cfg.DrainWait); ; wait = min(2*wait, 2*time.Second) {
		failed := &check{}
		doc := k.ok(failed, k.send(request{method: http.MethodGet, target: "/health"}), "health.json")
		if n, ok := num(doc["in_flight"]); ok && n == 0 && len(failed.fails) == 0 {
			return true
		}
		if time.Now().Add(wait).After(deadline) {
			return false
		}
		time.Sleep(wait)
	}
}

func (k *kit) applyBody(callID, fingerprint string, desired map[string]any) map[string]any {
	return map[string]any{"contract_version": Major, "call_id": callID, "subject": k.cfg.Subject, "email": k.cfg.Email,
		"fingerprint": fingerprint, "plan_id": "conformance-plan-" + callID, "actor": "conformance-kit", "desired": desired}
}

func (k *kit) planOne(c *check, desired map[string]any) map[string]any {
	ex := k.send(request{method: http.MethodPost, target: "/plan", body: k.planBody(desired), schema: "plan_request.json"})
	doc := k.ok(c, ex, "plan_response.json")
	if outs := objects(doc["outcomes"]); len(outs) == 1 {
		return outs[0]
	}
	if doc != nil {
		c.fail("a plan for one subject answered %d outcomes", len(objects(doc["outcomes"])))
	}
	return nil
}

// converge plans then applies, as the backend does, and holds the apply to
// what the plan said it would do.
func (k *kit) converge(c *check, callID string, desired map[string]any, creates bool) (*exchange, map[string]any) {
	apply := func(plan map[string]any) *request {
		return &request{method: http.MethodPost, target: "/apply", body: k.applyBody(callID, str(plan, "fingerprint"), desired),
			schema: "apply_request.json", creates: creates}
	}
	var plan map[string]any
	var ex *exchange
	if k.created.id == "" {
		// Before the kit owns an account, writes go out under owner()'s rule 1.
		if plan = k.planOne(c, desired); plan == nil || k.frozen() {
			return nil, nil
		}
		ex = k.send(*apply(plan))
	} else if plan, ex = k.toOwned(c, desired, apply); ex == nil {
		return nil, nil
	}
	out := k.ok(c, ex, "apply_ok.json")
	done := out
	if code := ex.code(); code == "APPLY_BLOCKED" || code == "BINDING_CONFLICT" {
		done = ex.doc()
	}
	if done != nil {
		want := map[string]string{"apply": "applied", "no_change": "no_change", "blocked": "blocked"}[str(plan, "effect")]
		k.at("plan-equals-apply").expect(str(done, "effect") == want && str(done, "consequence") == str(plan, "consequence"),
			"the plan said %s (%q), the apply did %s (%q)", str(plan, "effect"), str(plan, "consequence"), str(done, "effect"), str(done, "consequence"))
	}
	return ex, out
}

func (k *kit) accountIDs(c *check) []string {
	return idsOf(k.ok(c, k.send(request{method: http.MethodGet, target: "/subjects"}), "subjects.json"))
}

// state is the kit account's state as /subjects reports it: the truth, since
// holding is observed.
func (k *kit) state(c *check) map[string]any {
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/subjects"}), "subjects.json")
	for _, s := range objects(doc["subjects"]) {
		if k.owner(s, false) == ours {
			st, _ := s["state"].(map[string]any)
			current, _ := doc["current"].(bool)
			k.unlistedHidden("the kit's account", st, current)
			return st
		}
	}
	c.fail("/subjects does not list the kit's account %s", k.created.id)
	return nil
}

func (k *kit) withdrawWithoutAccount(c *check) {
	before := k.accountIDs(c)
	_, out := k.converge(c, k.callID(), map[string]any{k.m.account: false}, false)
	if out != nil {
		c.expect(str(out, "effect") == "no_change" && !names(out), "answered %s with account_id %s, want no_change and no account", str(out, "effect"), display(out))
	}
	after := k.accountIDs(c)
	c.expect(slices.Equal(before, after), "/subjects went from %v to %v: an account was created", before, after)
}

// staleRefusal is §5 rule 2 on a refusal that writes nothing: a withdraw with
// a wrong fingerprint is PLAN_STALE, and the same call_id re-sent after a
// re-plan is executed, not answered with the remembered 409.
func (k *kit) staleRefusal(c *check) {
	callID, desired := k.callID(), map[string]any{k.m.account: false}
	ex := k.send(request{method: http.MethodPost, target: "/apply", schema: "apply_request.json",
		body: k.applyBody(callID, "conformance-wrong-fingerprint", desired)})
	if k.refused(c, ex, 409, "PLAN_STALE", "a withdraw with a wrong fingerprint") == nil {
		return
	}
	plan := k.planOne(c, desired)
	if plan == nil {
		return
	}
	ex = k.send(request{method: http.MethodPost, target: "/apply", schema: "apply_request.json",
		body: k.applyBody(callID, str(plan, "fingerprint"), desired)})
	if out := k.ok(c, ex, "apply_ok.json"); out != nil {
		c.expect(str(out, "effect") == "no_change", "the call refused with PLAN_STALE, re-sent after a re-plan, answered %s", str(out, "effect"))
	}
}

func (k *kit) fingerprint(c *check) {
	logBefore, logOK := k.logRecords(c)
	before := k.accountIDs(c)
	body := k.applyBody(k.callID(), "", k.full())
	delete(body, "fingerprint")
	k.refused(c, k.send(request{method: http.MethodPost, target: "/apply", body: body}), 400, "FINGERPRINT_REQUIRED", "an apply with no fingerprint")
	body = k.applyBody(k.callID(), "conformance-wrong-fingerprint", k.full())
	k.refused(c, k.send(request{method: http.MethodPost, target: "/apply", body: body, schema: "apply_request.json"}), 409, "PLAN_STALE", "an apply with a wrong fingerprint")
	k.unwritten(c, logBefore, logOK, "a refused apply")
	c.expect(slices.Equal(before, k.accountIDs(c)), "a refused apply created an account")
}

func (k *kit) readBack(c *check, out map[string]any) {
	observed, ok := out["observed"].(map[string]any)
	if !ok {
		c.skip("the apply carried no observed (unverified)")
		return
	}
	state := k.state(c)
	for name, v := range observed {
		k.shape(c, "observed", name, v)
		c.expect(same(state[name], v), "%s: observed %v, then /subjects says %v", name, v, state[name])
	}
}

// limits judges each limit the creating apply gave as max(unlimited, 5 GiB)
// from what the account now holds. A limit absent from its state is one the
// add-on did not read, "not known", never "no limit" (AUTHORING.md#get-subjects):
// it is skipped and named, like limitEmpty's, and with none read the check is
// a skip, not a fail on a value nobody reported.
func (k *kit) limits(c *check) {
	limits := k.m.withRole("limit")
	if len(limits) == 0 {
		c.skip("the manifest declares no limit field")
		return
	}
	state := k.state(c)
	if state == nil { // state failed c already, or the run halted
		c.unjudged()
		return
	}
	judged := false
	for _, f := range k.m.fields {
		if f.Role != "limit" {
			continue
		}
		held, reported := state[f.Name]
		if !reported {
			c.skip("%s is not in the kit's account's state (not read), so what [%q, \"5368709120\"] combined to cannot be seen", f.Name, f.Unlimited)
			continue
		}
		judged = true
		c.expect(same(held, []string{f.Unlimited}), "%s given [%q, \"5368709120\"] holds %v, want [%q]: unlimited dominates",
			f.Name, f.Unlimited, held, f.Unlimited)
	}
	if !judged {
		c.unjudged()
	}
}

// limitEmpty sends each limit as [] after setting it to 5 GiB: no role sets
// it, so nothing is set, and the account keeps the limit it holds. Clearing
// it would read as "no limit" on most apps, which nobody decided. It judges
// only from a 5 GiB the add-on was seen to hold: from an unreported limit, or
// one the first apply did not set, [] keeping it proves nothing.
func (k *kit) limitEmpty(c *check) {
	const fiveGiB = "5368709120"
	limits := k.m.withRole("limit")
	if len(limits) == 0 {
		c.skip("the manifest declares no limit field")
	}
	judged := false
	for _, name := range limits {
		k.converge(c, k.callID(), map[string]any{k.m.account: true, name: []string{fiveGiB}}, false)
		state := k.state(c)
		before, reported := state[name]
		switch {
		case state == nil: // state failed c already
			continue
		case !reported:
			c.skip("%s is not in the kit's account's state (not read), so what [] does to it cannot be seen", name)
			continue
		}
		judged = true
		if !c.expect(same(before, []string{fiveGiB}), "%s: an apply giving it [%q] left it %v", name, fiveGiB, before) {
			continue
		}
		k.converge(c, k.callID(), map[string]any{k.m.account: true, name: []string{}}, false)
		after := k.state(c)[name]
		c.expect(same(after, before), "%s: an apply giving it [] changed it from %v to %v; [] sets nothing", name, before, after)
	}
	if !judged { // the applies and reads that found no limit are not a pass
		c.unjudged()
	}
}

// readOperations sends every available effect: read operation under one
// call_id, the first of them twice, and that call_id is the one the creating
// apply was recorded under. Syndra sends one call_id for every read, so a read
// ignores it: never recorded, held or checked for reuse, and each answers 200
// with its own reply, never CALL_ID_REUSED and never the apply's. Reads by
// contract, sent only for the kit's own subject, after a plan naming its
// account (viaOwned), like every other operation.
func (k *kit) readOperations(c *check, callID string) {
	reads := k.readOps()
	if len(reads) == 0 {
		c.skip("no effect: read operation is declared available")
		return
	}
	send := k.viaOwned(c)
	for _, id := range append([]string{reads[0]}, reads...) {
		ex := send(request{method: http.MethodPost, target: "/operations/" + id,
			body: k.operationBody(id, callID, map[string]any{}), schema: "operation_request.json"})
		if doc := k.ok(c, ex, "operation_response.json"); doc != nil {
			c.expect(str(doc, "operation") == id, "%s under the creating apply's call_id answered for %q", id, str(doc, "operation"))
		}
	}
}

// readOps is every effect: read operation the manifest declares available, in
// id order.
func (k *kit) readOps() []string {
	var reads []string
	for id, available := range k.m.ops {
		if available && k.c.operations[id].Effect == "read" {
			reads = append(reads, id)
		}
	}
	slices.Sort(reads)
	return reads
}

// results holds every read to the core rule: it answers one `result`, valid
// against the schema the registry names for the id (which also holds current
// false iff stale), or, for an id that declares its own, an object. A reply
// without `result` carries no reading.
//
// A read writes nothing, so a reads-only run judges it too (owned false): the
// same reads about the kit's subject, sent without the plan that precedes a
// write. A read about a subject the add-on has no account for refuses it (422
// OPERATION_FAILED, the registry's code for an operation that could not be
// done), which says nothing about its result's shape: that read is skipped,
// and named, and -mutate (owned true) reads the account it made. Only a read
// the registry marks `subject: true` is excused: one about the target
// (`subject: false`, health.get) refused at all, or any other code
// (TARGET_VERSION_UNSUPPORTED), fails.
//
// ponytail: the registry has no code for "no account for this subject", so
// 422 OPERATION_FAILED on a subject read stands for it. The add-on's own
// faults are not 422 (TrueNAS answers a store it cannot read 500
// STORE_UNREADABLE, boundAccount), but another subject-read refusal an
// add-on answers 422 OPERATION_FAILED is skipped here too, and judged only
// under -mutate; a dedicated code is the upgrade (an owner's decision).
func (k *kit) results(c *check, owned bool) {
	reads := k.readOps()
	if len(reads) == 0 {
		c.skip("no effect: read operation is declared available")
		return
	}
	send := k.send
	if owned {
		send = k.viaOwned(c)
	}
	var judged, skipped []string
	defer func() {
		if len(skipped) > 0 {
			c.skip("judged %s; skipped %s: no account for the kit's subject; -mutate reads the account it makes",
				orNone(judged), strings.Join(skipped, ", "))
		}
	}()
	for _, id := range reads {
		ex := send(request{method: http.MethodPost, target: "/operations/" + id,
			body: k.operationBody(id, k.callID(), map[string]any{}), schema: "operation_request.json"})
		if !owned && *k.c.operations[id].Subject && ex.err == nil && ex.status == http.StatusUnprocessableEntity && ex.code() == "OPERATION_FAILED" {
			skipped = append(skipped, id)
			continue
		}
		judged = append(judged, id)
		doc := k.ok(c, ex, "operation_response.json")
		if doc == nil {
			continue
		}
		result, ok := doc["result"].(map[string]any)
		if !c.expect(ok, "%s answered no result object", id) {
			continue
		}
		if schema := k.c.operations[id].Result; schema != "" {
			if err := k.c.validate(schema, result); err != nil {
				c.fail("%s: the result %v", id, err)
			}
		}
	}
}

func orNone(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

func (k *kit) replay(c *check, callID string, first *exchange) {
	logBefore, logOK := k.logRecords(c)
	body := k.applyBody(callID, "conformance-another-fingerprint", k.full())
	body["plan_id"] = "conformance-another-plan"
	owned := k.viaOwned(c)
	ex := owned(request{method: http.MethodPost, target: "/apply", body: body, schema: "apply_request.json"})
	c.expect(ex.status == first.status && bytes.Equal(ex.body, first.body),
		"a replay with another plan_id and fingerprint answered %d %s, not the recorded reply byte for byte", ex.status, ex.code())
	k.unwritten(c, logBefore, logOK, "the replay")

	other := k.applyBody(callID, "conformance-another-fingerprint", map[string]any{k.m.account: true})
	k.refused(c, owned(request{method: http.MethodPost, target: "/apply", body: other, schema: "apply_request.json"}), 422, "CALL_ID_REUSED", "the call_id with another desired")
	other = k.applyBody(callID, "conformance-another-fingerprint", k.full())
	other["subject"] = k.cfg.Subject + "-other"
	k.refused(c, k.send(request{method: http.MethodPost, target: "/apply", body: other, schema: "apply_request.json"}), 422, "CALL_ID_REUSED", "the call_id for another subject")
}

// untouched asserts every field that is not a switch is as it was.
func (k *kit) untouched(c *check, before, after map[string]any, why string) {
	for _, f := range k.m.fields {
		if f.Role != "account" && f.Role != "channel" {
			c.expect(same(before[f.Name], after[f.Name]), "%s: %s went from %v to %v", why, f.Name, before[f.Name], after[f.Name])
		}
	}
}

func (k *kit) absent(c *check) {
	before := k.state(c)
	k.converge(c, k.callID(), map[string]any{k.m.account: true}, false)
	k.untouched(c, before, k.state(c), "an apply naming the account alone")
}

func (k *kit) operationBody(name, callID string, params map[string]any) map[string]any {
	return map[string]any{"contract_version": Major, "call_id": callID, "operation": name, "subject": k.cfg.Subject,
		"actor": "conformance-kit", "params": params}
}

func canary() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "conformance-canary-" + hex.EncodeToString(b)
}

func (k *kit) operations(c *check) {
	secret := k.at("operations.secret")
	if !k.m.has("operations") {
		c.skip("feature operations not declared")
		secret.skip("feature operations not declared")
		return
	}
	send := k.viaOwned(c) // every write here: a plan naming the kit's account just before it
	ex := send(request{method: http.MethodPost, target: "/operations/conformance.unknown",
		body: k.operationBody("conformance.unknown", k.callID(), map[string]any{}), schema: "operation_request.json"})
	k.refused(c, ex, 404, "OPERATION_FAILED", "an unknown operation")
	if !k.m.ops["password.set"] {
		c.skip("password.set is not declared available: the kit calls no other operation")
		secret.skip("password.set is not declared available")
		return
	}
	const route = "/operations/password.set"
	ex = send(request{method: http.MethodPost, target: route, body: k.operationBody("password.set", k.callID(), map[string]any{}), schema: "operation_request.json"})
	c.expect(ex.status == 400, "password.set with no password answered %d %s, want 400", ex.status, ex.code())

	first, second := canary(), canary()
	k.since, k.canaries = len(k.log), []string{first, second}
	callID := k.callID()
	logFirst, firstOK := k.logRecords(k.at("health"))
	sent := send(request{method: http.MethodPost, target: route, body: k.operationBody("password.set", callID, map[string]any{"password": first}), schema: "operation_request.json"})
	if doc := k.ok(c, sent, "operation_response.json"); doc != nil {
		c.expect(str(doc, "outcome") == "succeeded", "password.set answered outcome %q", str(doc, "outcome"))
		k.passwordCall, k.passwordReply = callID, sent.body
	}
	k.advanced(k.at("health"), logFirst, firstOK, "the first password.set")
	logBefore, logOK := k.logRecords(c)
	again := send(request{method: http.MethodPost, target: route, body: k.operationBody("password.set", callID, map[string]any{"password": second}), schema: "operation_request.json"})
	c.expect(again.status == sent.status && bytes.Equal(again.body, sent.body),
		"the same call_id with another password answered %d %s, not the recorded reply: a secret parameter is inside the identity", again.status, again.code())
	k.unwritten(c, logBefore, logOK, "the re-send")

	probe := k.operationBody("password.set", k.callID(), map[string]any{"password": canary()})
	k.canaries = append(k.canaries, probe["params"].(map[string]any)["password"].(string))
	k.strict(k.at("strict-requests"), route, http.MethodPost, probe, send)
}

func (k *kit) lifecycle(c, refusal *check) {
	set := func(state, reason string) *exchange {
		return k.send(request{method: http.MethodPost, target: "/lifecycle", schema: "lifecycle_request.json",
			body: map[string]any{"contract_version": Major, "call_id": k.callID(), "state": state, "reason": reason}})
	}
	k.readOnly = true // before it is sent: restoring active is harmless, missing it is not
	if k.ok(c, set("read_only", "conformance kit: checking that writes are refused"), "lifecycle_response.json") == nil {
		k.cutShort("the add-on did not go read_only", "strict-requests", "replay.refusal")
		return
	}
	callID, wasRefused := k.callID(), false
	apply := func(plan map[string]any) *request {
		return &request{method: http.MethodPost, target: "/apply", schema: "apply_request.json",
			body: k.applyBody(callID, str(plan, "fingerprint"), map[string]any{k.m.account: true})}
	}
	if _, ex := k.toOwned(c, map[string]any{k.m.account: true}, apply); ex != nil {
		wasRefused = k.refused(c, ex, 503, "LIFECYCLE_REFUSAL", "an apply while read_only") != nil
		c.expect(ex.header.Get("Retry-After") != "", "LIFECYCLE_REFUSAL carries no Retry-After")
	}
	k.ok(c, k.send(request{method: http.MethodGet, target: "/subjects"}), "subjects.json")
	if h := k.ok(c, k.send(request{method: http.MethodGet, target: "/health"}), "health.json"); h != nil {
		c.expect(str(h, "lifecycle") == "read_only", "health says lifecycle %q while read_only", str(h, "lifecycle"))
	}
	k.strict(k.at("strict-requests"), "/lifecycle", http.MethodPost,
		map[string]any{"contract_version": Major, "call_id": k.callID(), "state": "read_only", "reason": "conformance kit: strict probe"}, nil)
	ex := k.send(request{method: http.MethodPost, target: "/lifecycle", body: map[string]any{"contract_version": Major, "call_id": k.callID(), "state": "active", "reason": " "}})
	k.refused(c, ex, 400, "REASON_REQUIRED", "a lifecycle change with a blank reason")
	ex = k.send(request{method: http.MethodPost, target: "/lifecycle", body: map[string]any{"contract_version": Major, "call_id": k.callID(), "state": "conformance-unknown", "reason": "conformance kit"}})
	k.refused(c, ex, 422, "UNKNOWN_LIFECYCLE_STATE", "an unknown lifecycle state")
	if k.ok(c, set("active", "conformance kit: done checking read_only"), "lifecycle_response.json") != nil {
		k.readOnly = false
	}
	if !wasRefused {
		k.cutShort("the apply sent while read_only was not refused, so there is no refusal to re-send (see lifecycle)", "replay.refusal")
		return
	}
	if _, ex := k.toOwned(refusal, map[string]any{k.m.account: true}, apply); ex != nil {
		refusal.expect(ex.status == 200, "the call refused while read_only, re-sent once active, answered %d %s: the refusal was recorded",
			ex.status, ex.code())
	}
}

func (k *kit) restoreActive() {
	if !k.readOnly {
		return
	}
	ex := k.send(request{method: http.MethodPost, target: "/lifecycle", schema: "lifecycle_request.json",
		body: map[string]any{"contract_version": Major, "call_id": k.callID(), "state": "active", "reason": "conformance kit: restoring active after an interrupted check"}})
	k.readOnly = ex.status != 200
}

// settingsWrite writes the first setting's current value back. A setting is
// target-wide, so this needs its own -mutate-settings, and it writes only a
// value it read from a reply that validated.
func (k *kit) settingsWrite(c *check) {
	if !k.m.has("settings") {
		return
	}
	if !k.cfg.MutateSettings {
		c.skip("the write-back touches a target-wide setting: needs -mutate-settings")
		return
	}
	failed := len(c.fails)
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/settings"}), "settings.json")
	list := objects(doc["settings"])
	var value string
	isString := false
	if len(list) > 0 {
		value, isString = list[0]["value"].(string)
	}
	if len(c.fails) > failed || !isString {
		c.skip("no setting read from a valid reply: nothing written back")
		return
	}
	target := "/settings/" + url.PathEscape(str(list[0], "name"))
	body := func() map[string]any {
		return map[string]any{"contract_version": Major, "call_id": k.callID(), "value": value, "actor": "conformance-kit"}
	}
	k.ok(c, k.send(request{method: http.MethodPut, target: target, body: body(), schema: "setting_put_request.json"}), "setting.json")
	k.strict(k.at("strict-requests"), target, http.MethodPut, body(), nil)
}

func (k *kit) withdrawExisting(c, channel *check) {
	// Hold a managed value first, so "every membership unchanged" is a claim
	// about something Syndra gave and not about an empty list.
	hold := map[string]any{k.m.account: true}
	for _, name := range k.m.withRole("membership") {
		if vals := k.values[name]; len(vals) > 0 {
			hold[name] = vals[:1]
		}
	}
	k.converge(c, k.callID(), hold, false)
	before := k.state(c)
	desired := map[string]any{k.m.account: false}
	for _, name := range k.m.withRole("channel") {
		desired[name] = true // asked for, and must still end off
	}
	_, out := k.converge(c, k.callID(), desired, false)
	after := k.state(c)
	c.expect(after[k.m.account] == false, "after a withdraw-only apply the account is %v", after[k.m.account])
	k.untouched(c, before, after, "a withdraw-only apply")
	k.created.off = after[k.m.account] == false
	observed, _ := out["observed"].(map[string]any)
	for _, name := range k.m.withRole("channel") {
		channel.expect(after[name] == false && (observed == nil || observed[name] == false),
			"desired {%s: false, %s: true} left %s %v (observed %v)", k.m.account, name, name, after[name], observed[name])
	}
	if !channel.ran {
		channel.skip("the manifest declares no channel field")
	}
}

// afterwards holds every reply of the run to the rules that apply to all of
// them: the error envelope, the required fields, and no secret coming back.
func (k *kit) afterwards() {
	fields, envelope, secret := k.at("required-fields"), k.at("error-envelope"), k.at("operations.secret")
	for i, ex := range k.log {
		if ex.err != nil || ex.status == 0 {
			continue
		}
		if len(k.canaries) > 0 && i >= k.since {
			secret.mark()
			for _, s := range k.canaries {
				leaked := bytes.Contains(ex.body, []byte(s))
				for name, vals := range ex.header {
					leaked = leaked || strings.Contains(name+strings.Join(vals, ","), s)
				}
				secret.expect(!leaked, "%s: the reply carries a secret parameter sent earlier", ex)
			}
		}
		if ex.status >= 300 {
			k.envelope(envelope, ex)
		} else {
			k.required(fields, ex)
		}
	}
	k.logAnchored(k.at("health"))
}

// logAnchored holds every /health the run read, in order, to what Syndra
// anchors the mutation log by: a count that never falls; the same count, the
// same head; a higher count, a new head. (Each reply was validated where it was
// read; health.json holds the head non-empty.)
func (k *kit) logAnchored(c *check) {
	var prev map[string]any
	for _, ex := range k.log {
		if path, _, _ := strings.Cut(ex.target, "?"); ex.err != nil || ex.status != http.StatusOK || ex.method != http.MethodGet || path != "/health" {
			continue
		}
		doc := ex.doc()
		head, _ := doc["log_head"].(string)
		n, isNum := num(doc["log_records"])
		if !isNum {
			continue
		}
		if prev != nil {
			was, _ := num(prev["log_records"])
			moved := head != prev["log_head"]
			c.expect(n >= was, "log_records fell from %d to %d", was, n)
			c.expect(n != was || !moved, "log_records stayed at %d and log_head changed: the log was rewritten", n)
			c.expect(n <= was || moved, "log_records rose from %d to %d and log_head stayed %s", was, n, head)
		}
		prev = doc
	}
}

func (k *kit) envelope(c *check, ex *exchange) {
	where := ex.String()
	mt, _, _ := mime.ParseMediaType(ex.header.Get("Content-Type"))
	c.expect(mt == "application/problem+json", "%s %d: Content-Type %q, want application/problem+json", where, ex.status, mt)
	doc := ex.doc()
	if !c.expect(doc != nil, "%s %d: the body is not a JSON object", where, ex.status) {
		return
	}
	if err := k.c.validate("error.json", doc); err != nil {
		c.fail("%s %d: %v", where, ex.status, err)
	}
	code := str(doc, "code")
	reg, known := k.c.errors[code]
	if !c.expect(known, "%s %d: code %q is not in registry/errors.json", where, ex.status, code) {
		return
	}
	status, _ := num(doc["status"])
	c.expect(slices.Contains(reg.Statuses, ex.status), "%s: %s answered %d, the registry allows %v", where, code, ex.status, reg.Statuses)
	c.expect(status == int64(ex.status), "%s: body status %v, HTTP %d", where, doc["status"], ex.status)
	c.expect(str(doc, "type") == "urn:syndra:addon:error:"+code, "%s: type %q does not name %s", where, str(doc, "type"), code)
	c.expect(str(doc, "title") == reg.Title, "%s: title %q, the registry's is %q", where, str(doc, "title"), reg.Title)
	if retry, has := doc["retry"]; has && reg.Retry != "by_status" {
		c.expect(retry == reg.Retry, "%s: %s says retry %v, the registry %q", where, code, retry, reg.Retry)
	}
	if reg.Retry == "retry_after" {
		c.expect(ex.header.Get("Retry-After") != "", "%s: %s promises Retry-After and sends none", where, code)
	}
}

// required reads the §4 table: what each reply must carry, present, because
// absence is never a value.
func (k *kit) required(c *check, ex *exchange) {
	doc := ex.doc()
	path, _, _ := strings.Cut(ex.target, "?")
	need := func(m map[string]any, where string, keys ...string) {
		for _, key := range keys {
			_, ok := m[key]
			c.expect(ok, "%s: %s carries no %q", ex, where, key)
		}
	}
	usable := func(m map[string]any, where string) {
		s, _ := m["standing"].(map[string]any)
		_, ok := s["usable"].(bool)
		c.expect(ok, "%s: %s has an account and no standing.usable", ex, where)
	}
	switch {
	case doc == nil:
	case ex.method == http.MethodGet && path == "/subjects":
		need(doc, "the reply", "taken_at", "current", "truncated")
		for _, s := range objects(doc["subjects"]) {
			usable(s, "subject "+display(s))
		}
	case ex.method == http.MethodPost && path == "/plan":
		need(doc, "the reply", "taken_at", "current", "truncated")
		for _, o := range objects(doc["outcomes"]) {
			need(o, "an outcome", "effect")
			if names(o) {
				usable(o, "an outcome")
			}
		}
	case ex.method == http.MethodPost && path == "/apply":
		need(doc, "the outcome", "effect")
		if names(doc) {
			usable(doc, "the outcome")
		}
	}
}

// logRecords reads health.log_records from a reply that validated.
func (k *kit) logRecords(c *check) (int64, bool) {
	failed := len(c.fails)
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/health"}), "health.json")
	n, ok := num(doc["log_records"])
	return n, ok && len(c.fails) == failed
}

// unwritten asserts log_records did not move across something that must write
// nothing. A count that cannot be read makes that unverified: a failure.
func (k *kit) unwritten(c *check, before int64, okBefore bool, what string) {
	after, okAfter := k.logRecords(c)
	if c.expect(okBefore && okAfter, "%s: log_records could not be read, so \"nothing written\" is unverified", what) {
		c.expect(after == before, "%s wrote: log_records %d to %d", what, before, after)
	}
}

// advanced asserts log_records moved across a write that landed: without it,
// every "nothing written" above would hold for a count that never moves.
func (k *kit) advanced(c *check, before int64, okBefore bool, what string) {
	after, okAfter := k.logRecords(c)
	if c.expect(okBefore && okAfter, "%s: log_records could not be read around it", what) {
		c.expect(after > before, "log_records stayed at %d across %s: it does not count writes", before, what)
	}
}
