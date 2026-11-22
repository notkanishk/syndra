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
//     share whose auditing records everyone is counted, from the first day
//     that began inside an unbroken chain of sightings of it doing so, a
//     chain broken only by seeing it narrowed or gone, or by two sightings
//     more than 3 x STATS_COLLECT_EVERY apart (statsMeta.Recording,
//     LastSeen, Every; coverSessions); any other share or day is a gap.
//     The days are re-read from the log until they are final, so a day the
//     NAS was unreachable is filled once it answers. A run catching up
//     first reads the oldest row the log keeps: a day it no longer keeps
//     is a final gap, and so is a day with more rows than one run may read.
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
		s.collectStatsOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

// collectStatsOnce is one run of the collector.
// No request waits on it, so its calls get collectCallTimeout each: a busy
// NAS paging its audit log is still answering.
func (s *server) collectStatsOnce(ctx context.Context) {
	ctx = withCallLimit(ctx, collectCallTimeout)
	now := s.stats.clock()
	snap, errAccounts := s.readSubjects(ctx)
	bound, errBound := s.boundUIDs(snap.Subjects) // unread, the run fails like any other
	shares, errShares := s.enabledShares(ctx)
	failed := errors.Join(errAccounts, errBound, errShares)
	s.collectSessions(ctx, now, snap, shares, bound, failed)
	mounts, errMounts := s.datasetMounts(ctx)
	s.collectUsage(ctx, now, snap, shares, mounts, bound, errors.Join(failed, errMounts))
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

// recordsEveryone is B1: a share's auditing records everyone when it is on
// and watches every account that can connect over SMB (smb, from
// smbMemberships), by the one rule a member's own coverage uses (watches).
// Otherwise its count would undercount, which is a gap and never a number.
// TrueNAS 25.10 refuses auditing with both lists empty, so a watch list
// naming a group everyone is in (builtin_users) is how "everyone" is said
// there (owner, on the NAS, 21 Nov 2026). Lists that narrow nothing need no
// accounts read.
//
// Its ceiling: user.query lists the NAS's local accounts only, never a
// directory service's (AD or LDAP). Were the NAS to join one, its users could
// connect over SMB unseen here, and a share would be claimed to record
// everyone while leaving them out: their group must then be on the watch
// list, or the share not claimed. Whether a directory service is configured
// is not read (directoryservices.status is not on readMethods, and its reply
// on 25.10 is UNVERIFIED; addon-contract tasks.md).
func (sh shareAudit) recordsEveryone(smb []map[string]bool) bool {
	if !sh.Enabled {
		return false
	}
	for _, memberOf := range smb {
		if !sh.watches(memberOf) {
			return false
		}
	}
	return true
}

// narrowed is whether either audit list could leave somebody out: only then
// does recordsEveryone need the accounts.
func (sh shareAudit) narrowed() bool { return len(sh.WatchList) > 0 || len(sh.IgnoreList) > 0 }

// auditConnects reads every successful CONNECT from `from` on, oldest first,
// and says whether it stopped at the page bound with more to read.
func (s *server) auditConnects(ctx context.Context, from time.Time) (rows []auditRow, capped bool, err error) {
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
		if err := s.nas.call(ctx, "audit.query", query, &batch); err != nil {
			return nil, false, err
		}
		rows = append(rows, batch...)
		if len(batch) < statsAuditPage {
			return rows, false, nil
		}
	}
	return rows, true, nil
}

// auditOldest is the day of the oldest SMB row the NAS still keeps, "" when
// it keeps none.
func (s *server) auditOldest(ctx context.Context) (string, error) {
	var rows []auditRow
	query := []any{map[string]any{"services": []string{"SMB"}, "query-filters": []any{},
		"query-options": map[string]any{"order_by": []string{"message_timestamp"}, "limit": 1}}}
	if err := s.nas.call(ctx, "audit.query", query, &rows); err != nil || len(rows) == 0 {
		return "", err
	}
	return s.stats.day(time.Unix(rows[0].Timestamp, 0)), nil
}

// collectSessions recounts every day from the watermark through today.
func (s *server) collectSessions(ctx context.Context, now time.Time, snap Snapshot, shares []smbShare, bound map[int64]string, failed error) {
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
	// count is the first day counted from the log. A day gone from it is a
	// final gap, never a count of what is left: one the NAS no longer keeps
	// (it purges its log past its own retention, so the day of the oldest
	// row it keeps may be part-purged), or one with more rows than a run may
	// read.
	count, gone := from, map[string]gapReason{}
	var audits []shareAudit
	var rows []auditRow
	var capped bool
	if failed == nil {
		audits, failed = s.readShareAudit(ctx)
	}
	var counted map[string]bool
	if failed == nil { // the settings were seen: the chain is judged now, whatever fails below
		counted, failed = s.coverSessions(ctx, &meta, audits, now)
	}
	if failed == nil && from < today { // catching up: is the log still there?
		var oldest string
		if oldest, failed = s.auditOldest(ctx); oldest >= from {
			for ; count <= oldest && count < today; count = nextDayKey(count) {
				gone[count] = reasonPurged
			}
		}
	}
	if failed == nil {
		rows, capped, failed = s.auditConnects(ctx, c.midnight(count))
	}
	if failed != nil {
		s.failedRun(metricSessions, meta, now, failed)
		return
	}

	covered := func(share, day string) bool {
		since, ok := meta.Recording[share]
		return ok && !c.midnight(day).Before(since)
	}
	// until is the first day not counted: past the page bound, the last
	// day read may be part-read, so it and every later day wait for the
	// next run, as one gap.
	until := nextDayKey(today)
	if capped && len(rows) > 0 {
		until = c.day(time.Unix(rows[len(rows)-1].Timestamp, 0))
		if until == count { // the bound read one day and not all of it: it would never move
			gone[count] = reasonOverfull
			count = nextDayKey(count)
			until = count
		}
	}
	days := map[string]statDay{}
	for d := from; d <= today; d = nextDayKey(d) {
		rec := statDay{}
		if why, ok := gone[d]; ok {
			rec.Gaps = []StatGap{gapOf(c.stamp(d), c.stamp(nextDayKey(d)), nil, why)}
		}
		for _, sh := range shares {
			if _, ok := gone[d]; !ok && !covered(sh.Name, d) && d < until {
				why := reasonAudited
				if counted[sh.Name] {
					why = reasonNotYet
					if i := slices.IndexFunc(gapReasons, func(r gapReason) bool { return r.text == meta.Before[sh.Name] }); i >= 0 {
						why = gapReasons[i]
					}
				}
				rec.Gaps = append(rec.Gaps, gapOf(c.stamp(d), c.stamp(nextDayKey(d)), map[string]string{"share": sh.Name}, why))
			}
		}
		days[d] = rec
	}
	uidOf := map[string]int64{}
	for _, sub := range snap.Subjects {
		uidOf[sub.Username] = sub.UID
	}
	type session struct{ day, who, share, id string }
	seen := map[session]bool{}
	totals := map[[2]string]int64{}  // day, share
	people := map[[3]string]int64{}  // day, share, subject
	unbound := map[string][]string{} // day → account keys
	for _, r := range rows {
		share, day := r.share(shares), c.day(time.Unix(r.Timestamp, 0))
		if r.Event != "CONNECT" || !r.Success || day < count || day >= until || share == "" || !covered(share, day) {
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
		rec.Points = append(rec.Points, StatPoint{Start: c.stamp(k[0]), End: c.stamp(nextDayKey(k[0])), Dimensions: map[string]string{"share": k[1]}, Value: n})
		days[k[0]] = rec
	}
	for k, n := range people {
		rec := days[k[0]]
		rec.Points = append(rec.Points, StatPoint{Start: c.stamp(k[0]), End: c.stamp(nextDayKey(k[0])), Subject: k[2],
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

// coverSessions judges, from this run's read of the share settings, each
// share's chain of sightings into meta, and says which shares record
// everyone. A share whose lists narrow it is checked against every SMB
// account; when they cannot be read it has no sighting this run, and the
// error fails the run (failedRun), as any unread NAS does.
//
// One rule: a share's coverage lasts across an unbroken chain of sightings
// (Recording the first, LastSeen the latest), and breaks only when a share is
// seen narrowed or no longer enabled, or when a sighting comes more than
// 3 x STATS_COLLECT_EVERY (the larger of this run's and the last sighting's,
// Every) after the last, whatever kept the runs between from seeing it.
func (s *server) coverSessions(ctx context.Context, meta *statsMeta, audits []shareAudit, now time.Time) (map[string]bool, error) {
	counted, unread := map[string]bool{}, map[string]bool{}
	var smb []map[string]bool
	var smbErr error
	if slices.ContainsFunc(audits, func(a shareAudit) bool { return a.Enabled && a.narrowed() }) {
		smb, smbErr = s.smbMemberships(ctx)
	}
	for _, a := range audits {
		if a.Enabled && a.narrowed() && smbErr != nil {
			unread[a.Name] = true // no sighting this run: the time rule decides
			continue
		}
		counted[a.Name] = a.recordsEveryone(smb)
	}
	for _, m := range []*map[string]time.Time{&meta.Recording, &meta.LastSeen} {
		if *m == nil {
			*m = map[string]time.Time{}
		}
	}
	if meta.Every == nil {
		meta.Every = map[string]time.Duration{}
	}
	if meta.Before == nil {
		meta.Before = map[string]string{}
	}
	end := func(name string) {
		delete(meta.Recording, name)
		delete(meta.LastSeen, name)
		delete(meta.Every, name)
		delete(meta.Before, name)
	}
	for name := range meta.Recording {
		if _, seen := counted[name]; !seen && !unread[name] { // no longer an enabled share
			end(name)
		}
	}
	for name, ok := range counted {
		if !ok {
			end(name)
			continue
		}
		last, sighted := meta.LastSeen[name]
		if _, held := meta.Recording[name]; !held || !sighted || now.Sub(last) > 3*max(meta.Every[name], s.stats.every) {
			delete(meta.Before, name) // a first sighting, or a store from before LastSeen: reasonNotYet
			if held && sighted {      // a gap in sightings: failed runs mean the NAS, none at all that nothing watched
				// ponytail: Failing is the run before this one, so a stop that followed a failed run reads as the NAS.
				meta.Before[name] = reasonUnwatched.text
				if meta.Failing {
					meta.Before[name] = reasonNAS.text
				}
			}
			meta.Recording[name] = now
		}
		meta.LastSeen[name], meta.Every[name] = now, s.stats.every
	}
	return counted, smbErr
}

// collectUsage samples today's levels: per share, and in home folders. A
// place's bytes are its datasets' as the live read resolves them (datasetAt
// on the same mounts), so a folder that is its own dataset counts where
// storage.status shows it.
func (s *server) collectUsage(ctx context.Context, now time.Time, snap Snapshot, shares []smbShare, mounts datasetMap, bound map[int64]string, failed error) {
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
			t = s.userQuotas(ctx, dataset) // nil when unreadable
			tables[dataset] = t
		}
		return t
	}
	point := func(subject string, dims map[string]string, v int64) StatPoint {
		return StatPoint{Start: c.stamp(today), End: c.stamp(nextDayKey(today)), Subject: subject, Dimensions: dims, Value: v}
	}
	unboundOf := func(uid int64, into map[string]bool) {
		if uid >= systemUIDCeiling { // the machine's own accounts are not people
			into[accountKey("", uid, true)] = true
		}
	}

	// Whose home is locked, by its share: one place's lock affects only
	// itself (owner, 21 Nov 2026), so each such person's own series there
	// says not counted, and nobody else's does.
	lockedHomeOf := map[string][]string{}
	for _, sub := range snap.Subjects {
		sh, inside, ok := shareAt(shares, sub.Home)
		if _, _, locked := homeDatasets(mounts, sub.Home); ok && inside != "" && locked {
			if subject, bound := bound[sub.UID]; bound {
				lockedHomeOf[sh.Name] = append(lockedHomeOf[sh.Name], subject)
			}
		}
	}
	dayGap := func(dims map[string]string, why gapReason) StatGap {
		return gapOf(c.stamp(today), c.stamp(nextDayKey(today)), dims, why)
	}

	// storage.used_bytes: every dataset a share reaches, nested ones
	// included, by everyone in them (usageUnder, the count storage.status
	// reads live). A share whose own dataset is unreadable is not sampled; one
	// that is a plain folder, or locked, is a gap and no sample, today's
	// earlier one dropped. One with a nested dataset that could not be read
	// is sampled with a gap over every series of it: whose bytes are missing
	// is not known. One with a nested dataset that is locked is sampled with
	// a gap over its total alone, and, where that dataset is a person's home,
	// one over that person's own series: everyone else's stand.
	used, usedUnbound := map[string][]StatPoint{}, map[string]bool{}
	usedGaps := map[string][]StatGap{}
	var usedDims []map[string]string
	for _, sh := range shares {
		dims := map[string]string{"share": sh.Name}
		usedDims = append(usedDims, dims)
		u := usageUnder(mounts, sh.Path, table)
		if !u.Own || u.Locked {
			why := reasonFolder
			if u.Own {
				why = reasonLocked
			}
			used[sh.Name] = nil
			usedGaps[sh.Name] = []StatGap{dayGap(dims, why)}
			continue
		}
		if u.Top == nil {
			continue // not read: the share keeps today's last sample
		}
		// The gaps name no folder: one is often named for its account, and
		// a gap's reason is read by everyone the share reaches.
		if n := len(u.Unread); n > 0 {
			usedGaps[sh.Name] = append(usedGaps[sh.Name], dayGap(dims, reasonPartOf([]string{foldersIn(sh.Name, n, "")})))
		}
		if n := len(u.LockedIn); n > 0 {
			usedGaps[sh.Name] = append(usedGaps[sh.Name], dayGap(dims, reasonPartOf([]string{foldersIn(sh.Name, n, " (locked)")})).forTotal())
			for _, subject := range lockedHomeOf[sh.Name] {
				usedGaps[sh.Name] = append(usedGaps[sh.Name], dayGap(dims, reasonHomeLocked).forSubject(subject))
			}
		}
		var total int64
		for uid, v := range u.ByUID {
			total += v
			if v == 0 {
				continue
			}
			if subject, ok := bound[uid]; ok {
				used[sh.Name] = append(used[sh.Name], point(subject, dims, v))
			} else {
				unboundOf(uid, usedUnbound)
			}
		}
		used[sh.Name] = append(used[sh.Name], point("", dims, total))
	}

	// storage.home_bytes: the datasets holding home folders (each home's
	// own, as the live read resolves it: homeDatasets), read whole, or not
	// sampled. A locked home affects only itself (owner, 21 Nov 2026): it
	// has no point and a gap over its person's own series, everyone else's
	// are sampled, and the total is partial, with a gap over the total alone
	// saying where. That gap names the folder's place, never the folder: a
	// home is named for its account, and no account's name is on the wire.
	var home []StatPoint
	homeUnbound := map[string]bool{}
	homeRead := true
	var homeTotal int64
	counted := map[string]bool{}
	var lockedHomes []string
	var homeGaps []StatGap
	for _, sub := range snap.Subjects {
		sh, inside, ok := shareAt(shares, sub.Home)
		if !ok || inside == "" {
			continue
		}
		dataset, _, locked := homeDatasets(mounts, sub.Home)
		if locked { // never read, never a 0
			where := sh.Name
			if dir := path.Dir(inside); dir != "." {
				where += " › " + dir
			}
			lockedHomes = append(lockedHomes, "a home folder in "+where+" (locked)")
			if subject, ok := bound[sub.UID]; ok {
				homeGaps = append(homeGaps, dayGap(nil, reasonHomeLocked).forSubject(subject))
			}
			continue
		}
		var t map[int64]PlaceUsage
		if dataset != "" {
			t = table(dataset)
		}
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
	if len(lockedHomes) > 0 {
		slices.Sort(lockedHomes)
		homeGaps = append(homeGaps, dayGap(nil, reasonPartOf(slices.Compact(lockedHomes))).forTotal())
	}

	s.putLevels(metricUsedBytes, metas[metricUsedBytes], now, usedDims, func(rec statDay) (statDay, bool) {
		kept := statDay{}
		for _, p := range rec.Points {
			if _, replaced := used[p.Dimensions["share"]]; !replaced {
				kept.Points = append(kept.Points, p) // a share not read now keeps today's last sample
			}
		}
		for _, g := range rec.Gaps {
			if _, replaced := used[g.Dimensions["share"]]; !replaced {
				kept.Gaps = append(kept.Gaps, g) // and its gap
			}
		}
		for name, ps := range used {
			kept.Points = append(kept.Points, ps...)
			kept.Gaps = append(kept.Gaps, usedGaps[name]...)
		}
		return kept, len(used) == len(shares)
	}, usedUnbound)
	s.putLevels(metricHomeBytes, metas[metricHomeBytes], now, []map[string]string{nil}, func(rec statDay) (statDay, bool) {
		if !homeRead {
			return rec, false
		}
		return statDay{Points: home, Gaps: homeGaps}, true
	}, homeUnbound)
}

// putLevels writes a gauge's run: today's record as sample makes it from
// what it held (and whether it replaced it whole), and the days since the
// watermark closed. Today's unbound accounts are this run's when it replaced
// the record, a level being the last sample; otherwise they add to it. A closed day with
// no total for a dimension it reports was never sampled: a final gap, since
// a level cannot be read after the fact.
func (s *server) putLevels(metric string, meta statsMeta, now time.Time, dims []map[string]string, sample func(statDay) (statDay, bool), unbound map[string]bool) {
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
	for d := from; d < today; d = nextDayKey(d) {
		rec := stored[d]
		for _, want := range dims {
			sampled := false // a total, or a gap saying why there is none
			for _, p := range rec.Points {
				sampled = sampled || p.Subject == "" && sameDims(p.Dimensions, want)
			}
			for _, g := range rec.Gaps {
				sampled = sampled || g.Subject == "" && sameDims(g.Dimensions, want) // a person's gap says nothing of the total
			}
			if !sampled {
				rec.Gaps = append(rec.Gaps, gapOf(c.stamp(d), c.stamp(nextDayKey(d)), want, reasonDark))
			}
		}
		days[d] = rec
	}
	rec, replaced := sample(stored[today])
	sortPoints(today, rec.Points)
	days[today] = rec
	if meta.Unbound == nil {
		meta.Unbound = map[string][]string{}
	}
	if replaced {
		delete(meta.Unbound, today)
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
