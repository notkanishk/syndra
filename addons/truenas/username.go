package main

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
)

// Deriving a TrueNAS username (design §11).
//
// The target generates nothing. No middleware method produces a username and
// `user.create` requires one, so this add-on derives it — from the Zitadel
// identity's primary email localpart, because that is the one stable,
// unique-by-construction handle the IdP guarantees. The webui's own generator
// read the full NAME, truncated at 8, and resolved collisions by refusing to;
// current master removed it.
//
// Derivation happens ONCE, at account creation, and the resulting name is
// recorded. Renaming a TrueNAS account disturbs its home directory, its ACL
// entries and its SMB identity, so a later email change must not rename an
// existing account. That makes this a recovery path for the common case rather
// than a guarantee: if the local store is lost, re-deriving recovers every
// subject whose email has not changed, and Syndra's recorded binding covers the
// rest. The recorded binding remains authoritative.

// usernameMaxLen is TrueNAS's limit. Names are built to fit it rather than
// truncated into it, because a suffix appended after truncation either
// overflows or, if the truncation is redone, can collide with the name it was
// meant to disambiguate.
const usernameMaxLen = 32

// suffixLen is the collision suffix, including its separator.
//
// Reserved BEFORE truncation, never appended after. Four characters of base32
// over a SHA-256 of the Zitadel user id is twenty bits — ample for a
// disambiguator that, on a single Workspace domain, should never fire at all.
const suffixLen = 5 // "_" + 4

// DeriveUsername produces the account name for a subject.
//
// Google Workspace is the sole IdP and already guarantees localpart uniqueness
// WITHIN one domain, so the collision suffix is a correctness backstop that
// should not fire in practice. That "should not" rests entirely on the
// single-domain assumption: if Zitadel ever federates a second Workspace
// domain, two people can hold the same localpart and the suffix becomes
// routine. It is built to be correct either way, and the claim that it stays
// dormant is conditional.
//
// `collides` reports whether a candidate is already taken by somebody who is
// not this subject. It is a function rather than a set because the answer is a
// live fact about the target.
func DeriveUsername(email, subjectID string, collides func(string) bool) string {
	base := normalizeLocalpart(email)
	if base == "" {
		// A localpart that normalizes to nothing usable — all-invalid
		// characters, or empty after stripping — falls back to a DETERMINISTIC
		// name derived from the subject id. Never a random or sequential one:
		// re-deriving after losing the local store has to produce the same
		// name, and a counter would produce a different one every time.
		base = "u" + digestSuffix(subjectID, 8)
	}

	// Reserve the suffix before truncating, so a name that needs both stays
	// within the limit and still disambiguates.
	candidate := clampUsername(base, usernameMaxLen)
	if collides == nil || !collides(candidate) {
		return candidate
	}
	stem := clampUsername(base, usernameMaxLen-suffixLen)
	return stem + "_" + digestSuffix(subjectID, 4)
}

// normalizeLocalpart lowercases, strips sub-addressing, and replaces every
// character outside TrueNAS's pattern.
//
// The pattern is `^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$`, so the first character is
// narrower than the rest — a name starting with a dot or a dash is invalid even
// though both are legal later, which is the edge a naive filter gets wrong.
func normalizeLocalpart(email string) string {
	local, _, _ := strings.Cut(email, "@")
	local = strings.ToLower(strings.TrimSpace(local))
	// Sub-addressing: `ada+lab@x.edu` and `ada@x.edu` are one mailbox and must
	// be one account, or a member gets a second one by using a tagged address.
	local, _, _ = strings.Cut(local, "+")

	var b strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		default:
			// Replaced rather than dropped, so two distinct localparts that
			// differ only in an invalid character do not normalize to one name.
			b.WriteRune('_')
		}
	}
	out := b.String()

	// Usability is judged on what the LOCALPART produced, before anything is
	// prefixed. A name of nothing but separators carries no identity, and
	// checking after the prefix would find the prefix's own letter and call
	// `---` usable — which it then clamps back down to a single `u` that every
	// such subject shares.
	if !strings.ContainsAny(out, "abcdefghijklmnopqrstuvwxyz0123456789") {
		return ""
	}
	// A leading character outside `[a-zA-Z0-9_]` is invalid. Prefixed rather
	// than trimmed: trimming `.ada` and `..ada` gives one name for two people.
	if c := out[0]; !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '_' {
		out = "u" + out
	}
	return out
}

// clampUsername truncates to n characters without leaving a trailing dot or
// dash.
//
// A name ending in `.` or `-` is legal to TrueNAS and confusing to everybody
// else, and it is what a blind truncation produces roughly one time in ten.
//
// `_` is deliberately NOT trimmed. It is what an out-of-pattern character was
// replaced BY, so trimming it would undo the replacement and merge two people
// whose localparts differ only there — `adá` and `ad` becoming one account. A
// trailing underscore is mildly ugly; a shared account is not a cosmetic
// problem.
func clampUsername(s string, n int) string {
	if n < 1 {
		n = 1
	}
	if len(s) > n {
		s = s[:n]
	}
	return strings.TrimRight(s, ".-")
}

// digestSuffix is a stable hash of the subject id, base32 without padding.
//
// A hash of the SUBJECT ID, never a counter. A counter depends on the order
// accounts were created, so re-deriving after losing the store would hand
// somebody else's suffix to this person — and the whole reason derivation is a
// recovery path is that it produces the same answer twice.
//
// Base32 rather than hex: the alphabet is inside TrueNAS's pattern once
// lowercased, and it carries more bits per character in a budget of four.
func digestSuffix(subjectID string, n int) string {
	sum := sha256.Sum256([]byte(subjectID))
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
	if len(encoded) < n {
		return encoded
	}
	return encoded[:n]
}

// ValidUsername reports whether a name satisfies TrueNAS's pattern.
//
// Written out rather than as a regexp so the first-character rule is visible:
// it is narrower than the rest, and that asymmetry is the part a reader
// otherwise misses.
func ValidUsername(s string) bool {
	if s == "" || len(s) > usernameMaxLen {
		return false
	}
	body := s
	if strings.HasSuffix(body, "$") {
		body = body[:len(body)-1]
		if body == "" {
			return false
		}
	}
	first := body[0]
	if !(first >= 'a' && first <= 'z') && !(first >= 'A' && first <= 'Z') &&
		!(first >= '0' && first <= '9') && first != '_' {
		return false
	}
	for i := 1; i < len(body); i++ {
		c := body[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '.', c == '-':
		default:
			return false
		}
	}
	return true
}
