// Package textsort is the one way the backend orders a list of names: the way a
// person reads them, which is the way ui/src/lib/sort.ts orders them. Case and
// accents do not matter ("netbird" sits among the Ns, "Émile" with the Es), and
// numbers are numbers ("Lab 2" before "Lab 10"). Byte order and strings.ToLower
// do neither, so a name sort that uses them disagrees with the screen.
//
// Lists whose order carries meaning (time, send order, relevance) and lists of
// ids or keys that only need a stable order (fingerprints) do not use this.
package textsort

import (
	"sort"
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"syndra/internal/locale"
)

// The same options and the same locale as the UI's Intl.Collator(displayLocale(),
// {sensitivity: "base", numeric: true}): the deployment's LOCALE (Und, the root
// order, when unset), because an order is a display too (in sv, å sorts after
// z). A collate.Collator reuses an internal buffer, so it is not safe to share
// without the lock.
var (
	mu       sync.Mutex
	collator = newCollator(locale.Language())
)

func newCollator(tag language.Tag) *collate.Collator {
	return collate.New(tag, collate.Loose, collate.Numeric)
}

// Compare orders two names: negative, zero or positive, like strings.Compare.
func Compare(a, b string) int {
	mu.Lock()
	defer mu.Unlock()
	return collator.CompareString(a, b)
}

// Less reports whether name a sorts before name b.
func Less(a, b string) bool { return Compare(a, b) < 0 }

// Strings sorts names in place.
func Strings(names []string) {
	sort.SliceStable(names, func(i, j int) bool { return Less(names[i], names[j]) })
}

// ByName sorts items in place by name. Items with equal names fall back to
// tie, so two "Lab" entries never swap between runs; a nil tie leaves equal
// names in the order they came.
func ByName[T any](items []T, name func(T) string, tie func(T) string) {
	sort.SliceStable(items, func(i, j int) bool {
		if c := Compare(name(items[i]), name(items[j])); c != 0 {
			return c < 0
		}
		return tie != nil && tie(items[i]) < tie(items[j])
	})
}
