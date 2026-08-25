package main

import (
	"strings"
	"testing"
)

// 6.3–6.6 — username derivation. Every case here is a silent-corruption bug if
// it goes the other way: two people sharing an account, or one person getting a
// second one.

func never(string) bool { return false }

func TestDerivationIsDeterministicAndAlwaysValid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		email string
		want  string
		why   string
	}{
		{"ordinary", "ada.lovelace@example.edu", "ada.lovelace",
			"the common case must produce the obvious name"},
		{"uppercase", "Ada.Lovelace@example.edu", "ada.lovelace",
			"case is not identity: two capitalisations are one mailbox"},
		{"sub-addressed", "ada+lab@example.edu", "ada",
			"a tagged address is the same mailbox, and must not buy a second account"},
		{"non-ascii", "adá@example.edu", "ad_",
			"characters outside the pattern are replaced, never dropped — dropping merges two people"},
		{"leading dot", ".ada@example.edu", "u.ada",
			"the first character is narrower than the rest, and trimming would merge .ada with ..ada"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveUsername(tc.email, "sub-1", never)
			if got != tc.want {
				t.Fatalf("got %q, want %q — %s", got, tc.want, tc.why)
			}
			if !ValidUsername(got) {
				t.Fatalf("%q does not satisfy the target's pattern", got)
			}
			if again := DeriveUsername(tc.email, "sub-1", never); again != got {
				t.Fatalf("derivation must be deterministic: %q then %q", got, again)
			}
		})
	}
}

// The whole point of a mailbox-derived name is that one mailbox is one account.
func TestSubAddressingCannotBuyASecondAccount(t *testing.T) {
	plain := DeriveUsername("ada@example.edu", "sub-1", never)
	tagged := DeriveUsername("ada+printing@example.edu", "sub-1", never)
	if plain != tagged {
		t.Fatalf("one mailbox must be one account: %q vs %q", plain, tagged)
	}
}

// Two localparts differing only in an invalid character are two people.
// Dropping the character rather than replacing it would merge them.
func TestInvalidCharactersAreReplacedRatherThanDropped(t *testing.T) {
	a := DeriveUsername("a b@example.edu", "sub-1", never)
	b := DeriveUsername("ab@example.edu", "sub-2", never)
	if a == b {
		t.Fatalf("dropping the invalid character merged two people onto %q", a)
	}

	// The same hazard at the END of a name, where a trailing-separator trim
	// would undo the replacement: `adá` and `ad` are two people.
	accented := DeriveUsername("adá@example.edu", "sub-1", never)
	plain := DeriveUsername("ad@example.edu", "sub-2", never)
	if accented == plain {
		t.Fatalf("trimming the replacement merged two people onto %q", plain)
	}
}

// A localpart that normalizes to nothing usable falls back to a deterministic
// name derived from the subject id — never a random or sequential one, because
// re-deriving after losing the local store has to produce the same answer.
func TestAnUnusableLocalpartFallsBackDeterministically(t *testing.T) {
	for _, email := range []string{"@example.edu", "...@example.edu", "+@example.edu", "---@example.edu"} {
		first := DeriveUsername(email, "sub-42", never)
		second := DeriveUsername(email, "sub-42", never)
		if first == "" || !ValidUsername(first) {
			t.Fatalf("%q produced %q, which is not a usable name", email, first)
		}
		if first != second {
			t.Fatalf("the fallback must be deterministic: %q then %q", first, second)
		}
		// And two subjects with equally unusable localparts must not collide.
		other := DeriveUsername(email, "sub-43", never)
		if other == first {
			t.Fatalf("two subjects collapsed onto one fallback name %q", first)
		}
	}
}

// The suffix is reserved BEFORE truncation. Appended after, it either overflows
// the limit or — if the truncation is redone — produces a name that collides
// with the one it was meant to disambiguate.
func TestANameNeedingBothTruncationAndASuffixStaysValidAndDistinct(t *testing.T) {
	long := strings.Repeat("verylongname", 4) + "@example.edu"

	plain := DeriveUsername(long, "sub-1", never)
	if len(plain) > usernameMaxLen {
		t.Fatalf("a long name must be clamped, got %d chars", len(plain))
	}

	taken := map[string]bool{plain: true}
	suffixed := DeriveUsername(long, "sub-2", func(s string) bool { return taken[s] })
	if len(suffixed) > usernameMaxLen {
		t.Fatalf("a truncated-and-suffixed name must still fit: %q is %d chars", suffixed, len(suffixed))
	}
	if suffixed == plain {
		t.Fatal("the suffix must actually disambiguate")
	}
	if !ValidUsername(suffixed) {
		t.Fatalf("%q does not satisfy the target's pattern", suffixed)
	}
}

// The suffix is a hash of the subject id, never a counter. A counter depends on
// creation order, so re-deriving after losing the store would hand somebody
// else's suffix to this person — and re-derivation existing at all depends on
// producing the same answer twice.
func TestAForcedCollisionResolvesReproduciblyAndNeverReusesAnotherSubjectsName(t *testing.T) {
	taken := map[string]bool{"ada": true}
	collides := func(s string) bool { return taken[s] }

	first := DeriveUsername("ada@example.edu", "sub-2", collides)
	second := DeriveUsername("ada@example.edu", "sub-2", collides)
	if first != second {
		t.Fatalf("collision resolution must be reproducible: %q then %q", first, second)
	}
	if first == "ada" {
		t.Fatal("it must not hand over the taken name")
	}

	// A third subject with the same localpart gets a different suffix again.
	third := DeriveUsername("ada@example.edu", "sub-3", collides)
	if third == first {
		t.Fatalf("two subjects resolved onto one name %q", first)
	}
}

// A truncation that leaves a trailing separator is legal to the target and
// confusing to everybody else, and it happens roughly one time in ten.
func TestATruncatedNameDoesNotEndInASeparator(t *testing.T) {
	// Engineered so the cut lands exactly on a separator: 8 * "abc." is 32
	// characters ending in a dot, which is the limit precisely.
	email := strings.Repeat("abc.", 8) + "@example.edu"
	if len(strings.Repeat("abc.", 8)) != usernameMaxLen {
		t.Fatalf("the fixture must be exactly the limit long, or it tests nothing")
	}
	got := DeriveUsername(email, "sub-1", never)
	// A trailing `_` is left alone deliberately: it is what an out-of-pattern
	// character was replaced by, and trimming it would merge two people.
	if strings.HasSuffix(got, ".") || strings.HasSuffix(got, "-") {
		t.Fatalf("%q ends in a separator", got)
	}
	if !ValidUsername(got) {
		t.Fatalf("%q is not valid", got)
	}
}

// The validator's first-character rule is narrower than the rest, which is the
// asymmetry a reader otherwise misses.
func TestTheValidatorMatchesTheTargetsPattern(t *testing.T) {
	for _, ok := range []string{"ada", "_ada", "a", "ada.lovelace", "ada-l", "machine$", "a1"} {
		if !ValidUsername(ok) {
			t.Errorf("%q must be valid", ok)
		}
	}
	for _, bad := range []string{"", ".ada", "-ada", "ada lovelace", "adá", "$", strings.Repeat("a", usernameMaxLen+1)} {
		if ValidUsername(bad) {
			t.Errorf("%q must be invalid", bad)
		}
	}
}
