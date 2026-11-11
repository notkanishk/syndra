package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"path"
	"slices"
	"strconv"
	"time"
)

// The statistics collector: the add-on's own reads of the NAS, on its own
// timer, into its own buffer (feature stats). It is a read path and nothing
// else: it calls only reads (NAS.call, counted in target_reads), never
// server.mutate, never the mutation log, never the read cache (fresher), so
// no collection can change what /subjects or a write sees.
//
// Each run reads the accounts and shares once, then:
//
//   - storage.sessions from the SMB audit log: one audit.query per run,
//     every successful CONNECT since the metric's watermark, ascending, in
//     pages of statsAuditPage rows, at most statsAuditPages of them. A
//     session is a distinct `session` per account, share and day. Only a
//     share whose auditing records everyone is counted; any other is a gap.
//     The days are re-read from the log until they are final, so a day the
//     NAS was unreachable is filled once it answers.
//   - storage.used_bytes and storage.home_bytes from the user quota tables
//     (one pool.dataset.get_quota per dataset): today's level, rewritten by
//     every run, so the day keeps its last sample. A level cannot be read
//     after the fact, so a day that ended with no sample becomes a gap.
//
// Usage is owned by uid: the account's name is read only to find its uid,
// and the uid finds the subject through the binding. Usage by an account no
// binding names counts in the totals and in unbound, never per person, and
// no account's name is ever on the wire.

var (
	// statsAuditPage and statsAuditPages bound what one run asks of the
	// NAS's audit log: 50 pages of 1000 rows. Variables only so a test can
	// make a page small.
	statsAuditPage  = 1000
	statsAuditPages = 50
)

// collectStats runs the collector now and then every `every` until ctx ends.
// Zero never runs it (STATS_COLLECT_EVERY=0, for a conformance run: its NAS
// reads move target_reads like any other).
func collectStats(ctx context.Context, s *server, every time.Duration) {
	if every <= 0 {
		return
	}
	for {
		s.collectStatsOnce()
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

// collectStatsOnce is one run of the collector.
func (s *server) collectStatsOnce() {
	now := s.stats.clock()
	bound, err := s.store.BoundUIDs()
	if err != nil {
		log.Printf("[STATS] the bindings could not be read, nothing collected: %v", err)
		return
	}
	snap, errAccounts := s.readSubjects()
	shares, errShares := s.enabledShares()
	failed := errors.Join(errAccounts, errShares)
	s.collectSessions(now, snap, shares, bound, failed)
	s.collectUsage(now, snap, shares, bound, failed)
}

// oldest is the first day the buffer keeps: the day whose end is
// statsBufferDays before now.
func (c statsClock) oldest(now time.Time) string { return c.day(now.AddDate(0, 0, -statsBufferDays)) }

// failedRun keeps a metric where it was: nothing read, so nothing moves, and
// the reply says from when (handleStats).
func (s *server) failedRun(metric string, meta statsMeta, now time.Time, why error) {
	log.Printf("[STATS] %s not collected, its watermark held at %s: %v", metric, meta.Watermark, why)
	meta.Failing = true
	if err := s.store.PutStats(metric, meta, nil, s.stats.oldest(now)); err != nil {
		log.Printf("[STATS] %s: %v", metric, err)
	}
}

// accountKey is who used the NAS, for counting: the uid, or a digest of a
// name the NAS no longer resolves. Kept in the store only.
func accountKey(username string, uid int64, known bool) string {
	if known {
		return strconv.FormatInt(uid, 10)
	}
	sum := sha256.Sum256([]byte(username))
	return "name:" + hex.EncodeToString(sum[:8])
}

// everyone is B1: a share's auditing records everyone only when it is on
// and narrowed by neither list. Otherwise its count would undercount, which
// is a gap and never a number.
func (sh shareAudit) everyone() bool {
	return sh.Enabled && len(sh.WatchList) == 0 && len(sh.IgnoreList) == 0
}

// auditConnects reads every successful CONNECT from `from` on, oldest first,
// and says whether it stopped at the page bound with more to read.
func (s *server) auditConnects(from time.Time) (rows []auditRow, capped bool, err error) {
	for page := range statsAuditPages {
		var batch []auditRow
		query := []any{map[string]any{
			"services": []string{"SMB"},
			"query-filters": []any{
				[]any{"event", "=", "CONNECT"}, []any{"success", "=", true},
				[]any{"message_timestamp", ">=", from.Unix()},
			},
			"query-options": map[string]any{"order_by": []string{"message_timestamp"},
				"offset": page * statsAuditPage, "limit": statsAuditPage},
		}}
		if err := s.nas.call("audit.query", query, &batch); err != nil {
			return nil, false, err
		}
		rows = append(rows, batch...)
		if len(batch) < statsAuditPage {
			return rows, false, nil
		}
	}
	return rows, true, nil
}

// collectSessions recounts every day from the watermark through today.
func (s *server) collectSessions(now time.Time, snap Snapshot, shares []smbShare, bound map[int64]string, failed error) {
	c := s.stats
	meta, _, err := s.store.StatsMeta(metricSessions)
	if err != nil {
		log.Printf("[STATS] %s: %v", metricSessions, err)
		return
	}
	today, from := c.day(now), meta.Watermark
	first := from == ""
	if first {
		from = today // the buffer begins with the first run: before it, a gap
	}
	var audits []shareAudit
	var rows []auditRow
	var capped bool
	if failed == nil {
		audits, failed = s.readShareAudit()
	}
	if failed == nil {
		rows, capped, failed = s.auditConnects(c.midnight(from))
	}
	if failed != nil {
		s.failedRun(metricSessions, meta, now, failed)
		return
	}

	counted := map[string]bool{} // the enabled shares whose auditing records everyone
	for _, a := range audits {
		counted[a.Name] = a.everyone()
	}
	// until is the first day not counted: past the page bound, the last
	// day read may be part-read, so it and every later day wait for the
	// next run, as one gap.
	until := c.next(today)
	if capped && len(rows) > 0 {
		until = c.day(time.Unix(rows[len(rows)-1].Timestamp, 0))
	}
	days := map[string]statDay{}
	for d := from; d <= today; d = c.next(d) {
		rec := statDay{}
		for _, sh := range shares {
			if !counted[sh.Name] && d < until {
				rec.Gaps = append(rec.Gaps, StatGap{Start: c.stamp(d), End: c.stamp(c.next(d)),
					Dimensions: map[string]string{"share": sh.Name}, Reason: reasonAudited})
			}
		}
		days[d] = rec
	}
	uidOf := map[string]int64{}
	for _, sub := range snap.Subjects {
		uidOf[sub.Username] = sub.UID
	}
	enabled := map[string]bool{}
	for _, sh := range shares {
		enabled[sh.Name] = true
	}
	type session struct{ day, who, share, id string }
	seen := map[session]bool{}
	totals := map[[2]string]int64{}  // day, share
	people := map[[3]string]int64{}  // day, share, subject
	unbound := map[string][]string{} // day → account keys
	for _, r := range rows {
		share, day := r.ServiceData.Service, c.day(time.Unix(r.Timestamp, 0))
		if r.Event != "CONNECT" || !r.Success || day < from || day >= until || !enabled[share] || !counted[share] {
			continue
		}
		uid, known := uidOf[r.Username]
		who, id := accountKey(r.Username, uid, known), r.Session
		if id == "" {
			id = r.AuditID // a row with no session is a session of its own
		}
		if k := (session{day, who, share, id}); seen[k] {
			continue
		} else {
			seen[k] = true
		}
		totals[[2]string{day, share}]++
		if subject, ok := bound[uid]; known && ok {
			people[[3]string{day, share, subject}]++
		} else if !slices.Contains(unbound[day], who) {
			unbound[day] = append(unbound[day], who)
		}
	}
	for k, n := range totals {
		rec := days[k[0]]
		rec.Points = append(rec.Points, StatPoint{Start: c.stamp(k[0]), End: c.stamp(c.next(k[0])), Dimensions: map[string]string{"share": k[1]}, Value: n})
		days[k[0]] = rec
	}
	for k, n := range people {
		rec := days[k[0]]
		rec.Points = append(rec.Points, StatPoint{Start: c.stamp(k[0]), End: c.stamp(c.next(k[0])), Subject: k[2],
			Dimensions: map[string]string{"share": k[1]}, Value: n})
		days[k[0]] = rec
	}
	if meta.Unbound == nil {
		meta.Unbound = map[string][]string{}
	}
	for d, rec := range days {
		sortPoints(d, rec.Points)
		delete(meta.Unbound, d)
		if len(unbound[d]) > 0 {
			meta.Unbound[d] = unbound[d]
		}
	}

	meta.Watermark, meta.Behind = today, ""
	if until <= today {
		meta.Watermark, meta.Behind = until, until
	}
	if first {
		meta.HeldFrom = today
	}
	meta.TakenAt, meta.Failing = now, false
	if err := s.store.PutStats(metricSessions, meta, days, c.oldest(now)); err != nil {
		log.Printf("[STATS] %s: %v", metricSessions, err)
	}
}

// collectUsage samples today's levels: per share, and in home folders.
func (s *server) collectUsage(now time.Time, snap Snapshot, shares []smbShare, bound map[int64]string, failed error) {
	c := s.stats
	today := c.day(now)
	metas := map[string]statsMeta{}
	for _, m := range []string{metricHomeBytes, metricUsedBytes} {
		meta, _, err := s.store.StatsMeta(m)
		if err != nil {
			log.Printf("[STATS] %s: %v", m, err)
			return
		}
		if failed != nil {
			s.failedRun(m, meta, now, failed)
			continue
		}
		metas[m] = meta
	}
	if failed != nil {
		return
	}
	tables := map[string]map[int64]PlaceUsage{}
	table := func(dataset string) map[int64]PlaceUsage {
		t, read := tables[dataset]
		if !read {
			t = s.userQuotas(dataset) // nil when unreadable
			tables[dataset] = t
		}
		return t
	}
	point := func(subject string, dims map[string]string, v int64) StatPoint {
		return StatPoint{Start: c.stamp(today), End: c.stamp(c.next(today)), Subject: subject, Dimensions: dims, Value: v}
	}
	unboundOf := func(uid int64, into map[string]bool) {
		if uid >= systemUIDCeiling { // the machine's own accounts are not people
			into[accountKey("", uid, true)] = true
		}
	}

	// storage.used_bytes: each share's dataset, by everyone in it.
	used, usedUnbound := map[string][]StatPoint{}, map[string]bool{}
	var usedDims []map[string]string
	for _, sh := range shares {
		dims := map[string]string{"share": sh.Name}
		usedDims = append(usedDims, dims)
		t := table(datasetOf(sh.Path))
		if t == nil {
			continue // not read: the share keeps today's last sample
		}
		var total int64
		for uid, u := range t {
			total += u.UsedBytes
			if u.UsedBytes == 0 {
				continue
			}
			if subject, ok := bound[uid]; ok {
				used[sh.Name] = append(used[sh.Name], point(subject, dims, u.UsedBytes))
			} else {
				unboundOf(uid, usedUnbound)
			}
		}
		used[sh.Name] = append(used[sh.Name], point("", dims, total))
	}

	// storage.home_bytes: the datasets holding home folders, read whole, or
	// not sampled.
	var home []StatPoint
	homeUnbound := map[string]bool{}
	homeRead := true
	var homeTotal int64
	counted := map[string]bool{}
	for _, sub := range snap.Subjects {
		if _, inside, ok := shareAt(shares, sub.Home); !ok || inside == "" {
			continue
		}
		dataset := datasetOf(path.Dir(sub.Home))
		t := table(dataset)
		if t == nil {
			homeRead = false
			continue
		}
		if !counted[dataset] {
			counted[dataset] = true
			for _, u := range t {
				homeTotal += u.UsedBytes
			}
		}
		if subject, ok := bound[sub.UID]; ok {
			home = append(home, point(subject, nil, t[sub.UID].UsedBytes)) // a level, 0 included: the meter needs one
		} else if t[sub.UID].UsedBytes > 0 {
			unboundOf(sub.UID, homeUnbound)
		}
	}
	home = append(home, point("", nil, homeTotal))

	s.putLevels(metricUsedBytes, metas[metricUsedBytes], now, usedDims, func(rec statDay) statDay {
		kept := rec.Points[:0]
		for _, p := range rec.Points {
			if _, replaced := used[p.Dimensions["share"]]; !replaced {
				kept = append(kept, p) // a share not read now keeps today's last sample
			}
		}
		for _, ps := range used {
			kept = append(kept, ps...)
		}
		return statDay{Points: kept}
	}, usedUnbound)
	s.putLevels(metricHomeBytes, metas[metricHomeBytes], now, []map[string]string{nil}, func(rec statDay) statDay {
		if !homeRead {
			return rec
		}
		return statDay{Points: home}
	}, homeUnbound)
}

// putLevels writes a gauge's run: today's record as sample makes it from
// what it held, and the days since the watermark closed. A closed day with
// no total for a dimension it reports was never sampled: a final gap, since
// a level cannot be read after the fact.
func (s *server) putLevels(metric string, meta statsMeta, now time.Time, dims []map[string]string, sample func(statDay) statDay, unbound map[string]bool) {
	c := s.stats
	today := c.day(now)
	stored := map[string]statDay{}
	from := meta.Watermark
	if from == "" {
		from, meta.HeldFrom = today, today
	}
	if err := s.store.StatsDays(metric, from, func(day string, rec statDay) bool {
		stored[day] = rec
		return true
	}); err != nil {
		log.Printf("[STATS] %s: %v", metric, err)
		return
	}
	days := map[string]statDay{}
	for d := from; d < today; d = c.next(d) {
		rec := stored[d]
		for _, want := range dims {
			sampled := false
			for _, p := range rec.Points {
				sampled = sampled || p.Subject == "" && sameDims(p.Dimensions, want)
			}
			if !sampled {
				rec.Gaps = append(rec.Gaps, StatGap{Start: c.stamp(d), End: c.stamp(c.next(d)), Dimensions: want, Reason: reasonDark})
			}
		}
		days[d] = rec
	}
	rec := sample(stored[today])
	sortPoints(today, rec.Points)
	days[today] = rec
	if meta.Unbound == nil {
		meta.Unbound = map[string][]string{}
	}
	for key := range unbound {
		if !slices.Contains(meta.Unbound[today], key) {
			meta.Unbound[today] = append(meta.Unbound[today], key)
		}
	}
	meta.Watermark, meta.TakenAt, meta.Failing = today, now, false
	if err := s.store.PutStats(metric, meta, days, c.oldest(now)); err != nil {
		log.Printf("[STATS] %s: %v", metric, err)
	}
}
