package conformance

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// manifest is what the kit needs of GET /capabilities.
type manifest struct {
	kind     string
	features []string
	fields   []struct{ Name, Role, Unlimited string }
	account  string
	ops      map[string]bool // id → available
	perPlan  int64           // limits.subjects_per_plan, 0 when undeclared
}

func (m *manifest) has(feature string) bool { return slices.Contains(m.features, feature) }

func (m *manifest) role(name string) string {
	for _, f := range m.fields {
		if f.Name == name {
			return f.Role
		}
	}
	return ""
}

func (m *manifest) withRole(role string) []string {
	var out []string
	for _, f := range m.fields {
		if f.Role == role {
			out = append(out, f.Name)
		}
	}
	return out
}

// reads runs every check that sends no write. /plan is among them: it mutates
// nothing by contract.
func (k *kit) reads() {
	k.vectors(k.at("transport.vectors"))
	caps := k.send(request{method: http.MethodGet, target: "/capabilities"})
	if caps.err != nil {
		k.at("transport.pin").fail("GET /capabilities: %v", caps.err)
		k.halted = "not reached: the transport did not connect (transport.pin)"
		return
	}
	k.at("transport.pin").expect(true, "")
	if k.m = k.manifest(k.at("manifest"), caps); k.m == nil {
		k.halted = "not reached: the manifest is unusable (manifest)"
		return
	}
	k.transport(k.at("transport.refusals"), k.at("transport.query"))
	health := k.health(k.at("health"))
	k.subjects(k.at("subjects"))
	k.at("subjects.paged").skip("feature subjects.paged is deferred (decision 27); no add-on declares it yet")
	k.current(k.at("current"))
	k.valuesCheck(k.at("values"))
	if c := k.at("settings"); !k.m.has("settings") {
		c.skip("feature settings not declared")
	} else {
		k.ok(c, k.send(request{method: http.MethodGet, target: "/settings"}), "settings.json")
	}
	k.plan(k.at("plan"))
	k.strictPlan(k.at("strict-requests"))
	ex := k.send(request{method: http.MethodGet, target: "/conformance/no-such-route"})
	k.refused(k.at("error-envelope"), ex, 404, "UNKNOWN_ROUTE", "an unknown route")

	k.at("operations.physical").skip("feature calls: its checks are slice 6")
	if k.m.has("freshness") {
		k.at("freshness").skip("declared; its checks are slice 6")
	} else {
		k.at("freshness").skip("feature freshness not declared")
	}
	k.scopes(k.at("dangerous-scopes"), health)
}

// vectors proves the kit's own transport before it judges anyone else's.
func (k *kit) vectors(c *check) {
	var v struct {
		Secret      string `json:"secret_utf8"`
		Derivations []struct {
			Target string `json:"target"`
			Public string `json:"tls_public_key_hex"`
			HMAC   string `json:"hmac_key_hex"`
		} `json:"derivations"`
		Raw struct {
			Vectors []struct {
				Name   string `json:"name"`
				Method string `json:"method"`
				Target string `json:"target"`
				Unix   string `json:"unix"`
				Body   string `json:"body_utf8"`
				HMAC   string `json:"hmac_key_hex"`
				MAC    string `json:"mac_hex"`
				Path   string `json:"path_as_sent"`
				Query  string `json:"raw_query"`
			} `json:"vectors"`
		} `json:"raw_query_signatures"`
	}
	if err := json.Unmarshal(k.c.vectors, &v); err != nil || len(v.Derivations) < 2 || len(v.Raw.Vectors) < 4 {
		c.fail("transport_derivation.json is unreadable or lost vectors: %v", err)
		return
	}
	for _, d := range v.Derivations {
		pub, key, err := deriveKeys([]byte(v.Secret), d.Target)
		c.expect(err == nil && hex.EncodeToString(pub) == d.Public && hex.EncodeToString(key) == d.HMAC,
			"derivation for %q does not reproduce", d.Target)
	}
	for _, vec := range v.Raw.Vectors {
		u, err := url.Parse("https://addon.example.org" + vec.Path + "?" + vec.Query)
		if vec.Query == "" {
			u, err = url.Parse("https://addon.example.org" + vec.Path)
		}
		c.expect(err == nil && signedTarget(u) == vec.Target, "%s: the kit signs %q, the vector %q", vec.Name, signedTarget(u), vec.Target)
		key, _ := hex.DecodeString(vec.HMAC)
		unix, _ := strconv.ParseInt(vec.Unix, 10, 64)
		c.expect(mac(unix, vec.Method, vec.Target, []byte(vec.Body), key) == vec.MAC, "%s: MAC does not reproduce", vec.Name)
	}
}

func (k *kit) manifest(c *check, ex *exchange) *manifest {
	k.unjudged(ex)
	doc := k.ok(c, ex, "manifest.json")
	if doc == nil {
		return nil
	}
	m := &manifest{kind: str(doc, "kind"), ops: map[string]bool{}}
	m.features, _ = stringList(doc["features"])
	n, _ := num(doc["contract_version"])
	c.expect(n == Major, "contract_version %v, the kit speaks %d", doc["contract_version"], Major)
	_, known := k.c.kinds[m.kind]
	c.expect(known, "kind %q is not in registry/kinds.json", m.kind)
	seen := map[string]bool{}
	for _, f := range objects(doc["entitlement_schema"]) {
		name, role := str(f, "name"), str(f, "role")
		c.expect(!seen[name], "field %q is declared twice", name)
		seen[name] = true
		m.fields = append(m.fields, struct{ Name, Role, Unlimited string }{name, role, str(f, "unlimited")})
	}
	accounts := m.withRole("account")
	if !c.expect(len(accounts) == 1, "%d fields have role account, want exactly one", len(accounts)) {
		return nil
	}
	m.account = accounts[0]
	for _, op := range objects(doc["operations"]) {
		_, registered := k.c.operations[str(op, "id")]
		c.expect(registered, "operation %q is not in registry/operations.json", str(op, "id"))
		m.ops[str(op, "id")], _ = op["available"].(bool)
	}
	c.expect(len(m.ops) == 0 || m.has("operations"), "operations are declared without feature operations")
	for _, f := range m.features {
		if need := k.c.features[f].RequiresKind; need != "" {
			c.expect(need == m.kind, "feature %s needs kind %s, the manifest says %s", f, need, m.kind)
		}
	}
	if limits, _ := doc["limits"].(map[string]any); limits != nil {
		m.perPlan, _ = num(limits["subjects_per_plan"])
	}
	return m
}

// transport sends what an attacker on the network could, and wants 401 for
// all of it.
func (k *kit) transport(refusals, query *check) {
	var bodies []string
	for _, s := range []struct {
		how  signing
		what string
	}{{unsigned, "unsigned"}, {staleClock, "signed ten minutes ago"}, {wrongKey, "signed with the wrong key"}} {
		ex := k.send(request{method: http.MethodGet, target: "/capabilities", sign: s.how})
		if d := k.refused(refusals, ex, 401, "UNAUTHENTICATED", s.what); d != nil {
			bodies = append(bodies, str(d, "title")+"|"+str(d, "detail"))
		}
	}
	if len(bodies) == 3 {
		refusals.expect(bodies[0] == bodies[1] && bodies[1] == bodies[2],
			"the three refusals differ (%q), which tells a caller which half failed", bodies)
	}

	ex := k.send(request{method: http.MethodGet, target: "/capabilities?conformance=signed"})
	query.expect(ex.err == nil && ex.status != 401, "a request signed over its query was refused (%d %s): "+
		"the add-on signs the path alone, as major 1 did", ex.status, ex.code())
	for _, alt := range []struct{ sent, signed, what string }{
		{"/capabilities?b=2&a=1", "/capabilities?a=1&b=2", "its query reordered"},
		{"/capabilities?a=2", "/capabilities?a=1", "its query altered"},
		{"/capabilities?a=1", "/capabilities", "a query added after signing"},
	} {
		ex := k.send(request{method: http.MethodGet, target: alt.sent, signAs: alt.signed})
		k.refused(query, ex, 401, "UNAUTHENTICATED", "a signed request with "+alt.what)
	}
}

func (k *kit) health(c *check) map[string]any {
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/health"}), "health.json")
	if doc == nil {
		return nil
	}
	_, isNum := num(doc["log_records"])
	_, isStr := doc["log_head"].(string)
	c.expect(isNum && isStr, "log_head %v and log_records %v must be a string and an integer", doc["log_head"], doc["log_records"])
	if k.m.has("freshness") {
		_, ok := num(doc["target_reads"])
		c.expect(ok, "feature freshness is declared and health carries no target_reads")
	}
	return doc
}

// subjects reads /subjects and checks every state against the schema's roles.
func (k *kit) subjects(c *check) map[string]any {
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/subjects"}), "subjects.json")
	if doc == nil {
		return nil
	}
	_, stale := doc["stale"]
	c.expect(doc["current"] != false || stale, "current is false and no stale says why")
	c.expect(doc["current"] != true || !stale, "current is true and stale is present")
	ids := map[string]bool{}
	for _, s := range objects(doc["subjects"]) {
		id := display(s)
		c.expect(!ids[id], "account_id %q is listed twice", id)
		ids[id] = true
		state, _ := s["state"].(map[string]any)
		for name, v := range state {
			k.shape(c, "subject "+id+" state", name, v)
		}
	}
	return doc
}

// shape checks one field value against its role's wire shape (design §3).
func (k *kit) shape(c *check, where, name string, v any) {
	role := k.m.role(name)
	if !c.expect(role != "", "%s: %q is not a field of the manifest", where, name) {
		return
	}
	if role == "account" || role == "channel" {
		_, ok := v.(bool)
		c.expect(ok, "%s: %s has role %s and must be true or false, is %v", where, name, role, v)
		return
	}
	list, ok := stringList(v)
	if !c.expect(ok, "%s: %s has role %s and must be a list of strings, is %v", where, name, role, v) {
		return
	}
	for _, s := range list {
		_, err := strconv.ParseUint(s, 10, 64)
		c.expect(role != "flag" || s == "true", `%s: %s is a flag, ["true"] or [], not %q`, where, name, s)
		c.expect(role != "limit" || err == nil, "%s: %s is a limit, %q is not a whole number", where, name, s)
	}
	c.expect(role != "membership" || (sort.StringsAreSorted(list) && len(slices.Compact(slices.Clone(list))) == len(list)),
		"%s: membership %s must be sorted and deduplicated, is %v", where, name, list)
}

// current reads twice, CurrentGap apart. A read that began after each request
// arrived cannot report the same moment twice, so two current:true answers
// with taken_at not moving forward are a mirror answering as if it were live.
func (k *kit) current(c *check) {
	type pair struct {
		target string
		body   any
		schema string
	}
	reads := []pair{{"/subjects", nil, ""}, {"/plan", k.planBody(map[string]any{k.m.account: false}), "plan_request.json"}}
	// The storage profile's reads carry current too (design §4's table), and
	// a cache served as current is the same mirror answering as if it were
	// live: held to the same rule when the feature is declared.
	if k.m.has("storage.reach") {
		reads = append(reads, pair{"/reach", nil, ""})
	}
	if k.m.has("storage.homes") {
		reads = append(reads, pair{"/homes", nil, ""})
	}
	first := make([]map[string]any, len(reads))
	for i, r := range reads {
		first[i] = k.readFor(r.target, r.body, r.schema)
	}
	time.Sleep(k.cfg.CurrentGap)
	for i, r := range reads {
		second := k.readFor(r.target, r.body, r.schema)
		if first[i] == nil || second == nil || first[i]["current"] != true || second["current"] != true {
			c.expect(first[i] != nil && second != nil, "%s did not answer twice", r.target)
			continue
		}
		t1, e1 := time.Parse(time.RFC3339Nano, str(first[i], "taken_at"))
		t2, e2 := time.Parse(time.RFC3339Nano, str(second, "taken_at"))
		c.expect(e1 == nil && e2 == nil && t2.After(t1),
			"%s: two reads %v apart both say current: true, taken_at %s then %s: the second is not a read that began after its request",
			r.target, k.cfg.CurrentGap, str(first[i], "taken_at"), str(second, "taken_at"))
	}
}

func (k *kit) readFor(target string, body any, schema string) map[string]any {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	ex := k.send(request{method: method, target: target, body: body, schema: schema})
	if ex.status != 200 {
		return nil
	}
	return ex.doc()
}

func (k *kit) valuesCheck(c *check) {
	for _, f := range k.m.fields {
		doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/values/" + url.PathEscape(f.Name)}), "values.json")
		if doc == nil {
			continue
		}
		vals, _ := stringList(doc["values"])
		c.expect(str(doc, "field") == f.Name, "/values/%s answers for field %q", f.Name, str(doc, "field"))
		c.expect(sort.StringsAreSorted(vals), "/values/%s is not sorted: %v", f.Name, vals)
		if f.Role == "account" || f.Role == "channel" {
			c.expect(doc["enumerable"] == true && slices.Equal(vals, []string{"false", "true"}),
				"/values/%s (role %s) must answer enumerable [false true], answers %v", f.Name, f.Role, vals)
		}
		if doc["enumerable"] == true {
			k.values[f.Name] = vals
		}
	}
	k.refused(c, k.send(request{method: http.MethodGet, target: "/values/conformance-no-such-field"}), 404, "UNKNOWN_FIELD", "an unknown field")
}

// planBody is a plan for the kit's subject alone.
func (k *kit) planBody(desired map[string]any) map[string]any {
	return map[string]any{"contract_version": Major, "subjects": []any{
		map[string]any{"subject": k.cfg.Subject, "email": k.cfg.Email, "desired": desired}}}
}

// full is the desired state that creates the kit's account: every role
// exercised, the limit given as max(unlimited, 5 GiB).
func (k *kit) full() map[string]any {
	d := map[string]any{}
	for _, f := range k.m.fields {
		switch f.Role {
		case "account", "channel":
			d[f.Name] = true
		case "membership":
			d[f.Name] = []string{}
			if vals := k.values[f.Name]; len(vals) > 0 {
				d[f.Name] = vals[:1]
			}
		case "flag":
			d[f.Name] = []string{"true"}
		case "limit":
			d[f.Name] = []string{f.Unlimited, "5368709120"}
		}
	}
	return d
}

func (k *kit) plan(c *check) {
	ex := k.send(request{method: http.MethodPost, target: "/plan", body: k.planBody(k.full()), schema: "plan_request.json"})
	if doc := k.ok(c, ex, "plan_response.json"); doc != nil {
		outs := objects(doc["outcomes"])
		if c.expect(len(outs) == 1 && str(outs[0], "subject") == k.cfg.Subject, "a plan for one subject answered %d outcomes", len(outs)) {
			c.expect(str(outs[0], "fingerprint") != "", "the outcome carries no fingerprint")
		}
	}
	if k.m.perPlan == 0 {
		c.skip("limits.subjects_per_plan undeclared: PLAN_TOO_LARGE not probed")
		return
	}
	if k.m.perPlan > 1000 {
		c.skip("limits.subjects_per_plan %d is too large to probe", k.m.perPlan)
		return
	}
	var many []any
	for i := range k.m.perPlan + 1 {
		many = append(many, map[string]any{"subject": fmt.Sprintf("%s-%d", k.cfg.Subject, i), "email": "", "desired": map[string]any{k.m.account: false}})
	}
	ex = k.send(request{method: http.MethodPost, target: "/plan", body: map[string]any{"contract_version": Major, "subjects": many}, schema: "plan_request.json"})
	if d := k.refused(c, ex, 422, "PLAN_TOO_LARGE", fmt.Sprintf("%d subjects", k.m.perPlan+1)); d != nil {
		limit, _ := num(d["limit"])
		c.expect(limit == k.m.perPlan, "PLAN_TOO_LARGE says limit %v, the manifest %d", d["limit"], k.m.perPlan)
	}
}

// strictPlan probes the strict decoder and the version check on /plan, which
// writes nothing even if a broken add-on accepts the probe.
func (k *kit) strictPlan(c *check) {
	k.strict(c, "/plan", http.MethodPost, map[string]any{"contract_version": Major, "subjects": []any{}}, nil)
	nested := map[string]any{"contract_version": Major, "subjects": []any{
		map[string]any{"subject": k.cfg.Subject, "email": "", "desired": map[string]any{}, "conformance_unknown_field": true}}}
	k.refused(c, k.send(request{method: http.MethodPost, target: "/plan", body: nested}), 400, "UNKNOWN_REQUEST_FIELD", "an unknown field inside a subject")
}

// strict sends valid with an unknown field added, with contract_version 1, and
// without contract_version, through via (k.send when nil).
func (k *kit) strict(c *check, target, method string, valid map[string]any, via func(request) *exchange) {
	if via == nil {
		via = k.send
	}
	with := func(edit func(map[string]any)) map[string]any {
		b := map[string]any{}
		for key, v := range valid {
			b[key] = v
		}
		edit(b)
		return b
	}
	ex := via(request{method: method, target: target, body: with(func(b map[string]any) { b["conformance_unknown_field"] = true })})
	k.refused(c, ex, 400, "UNKNOWN_REQUEST_FIELD", "an unknown field")
	for _, v := range []any{1, nil} {
		ex := via(request{method: method, target: target, body: with(func(b map[string]any) {
			if b["contract_version"] = v; v == nil {
				delete(b, "contract_version")
			}
		})})
		d := k.refused(c, ex, 422, "CONTRACT_VERSION_MISMATCH", fmt.Sprintf("contract_version %v", v))
		if d == nil {
			continue
		}
		addon, _ := num(d["addon_version"])
		_, hasCaller := num(d["caller_version"])
		c.expect(addon == Major && hasCaller, "CONTRACT_VERSION_MISMATCH must carry both numbers, carries caller_version %v, addon_version %v",
			d["caller_version"], d["addon_version"])
	}
}

func (k *kit) scopes(c *check, health map[string]any) {
	scopes, ok := stringList(health["credential_scopes"])
	if !ok {
		c.skip("health reports no credential_scopes")
		return
	}
	for _, s := range scopes {
		c.expect(!slices.Contains(k.c.kinds[k.m.kind].ScopeDenylist, s),
			"the credential holds %q, which kind %s denies: the operator should reissue it without", s, k.m.kind)
	}
	c.mark()
}

// unjudged takes out of the manifest the top-level keys this kit's schema does
// not know when the manifest declares a feature this kit does not know: such a
// key is that feature's, newer than the kit, and is skipped, never failed
// (design §2). A typo stays a failure, because it comes with no unknown feature.
// ponytail: top level only; a key a future feature adds inside a field object
// still fails here, until a feature introduces one and features.json lists it.
func (k *kit) unjudged(ex *exchange) {
	doc := ex.doc()
	features, _ := stringList(doc["features"])
	unknown := slices.DeleteFunc(features, func(f string) bool { _, known := k.c.features[f]; return known })
	schema, err := k.c.schema("manifest.json")
	if len(unknown) == 0 || err != nil {
		return
	}
	for key := range doc {
		if _, known := schema.Properties[key]; !known && !strings.HasPrefix(key, "x-") {
			k.skipped = append(k.skipped, key)
			delete(doc, key)
		}
	}
	slices.Sort(k.skipped)
	ex.body, _ = json.Marshal(doc)
}
