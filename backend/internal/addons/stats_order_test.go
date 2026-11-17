package addons

import (
	"slices"
	"testing"
)

// A-23: counters before levels, then the manifest's order, never the store's
// alphabetical one; a metric the manifest dropped goes last, by name.
func TestSortMetricsCountersThenManifestOrder(t *testing.T) {
	register(t, Registration{Target: "sorted"})
	t.Cleanup(func() { registryMu.Lock(); delete(registry, "sorted"); registryMu.Unlock() })
	g := func(n string) Metric { return Metric{Name: n, Instrument: InstrumentGauge} }
	c := func(n string) Metric { return Metric{Name: n, Instrument: InstrumentCounter} }
	registryMu.Lock()
	registry["sorted"].manifest = &Manifest{}
	registry["sorted"].metrics = []Metric{c("z.count"), g("m.level"), g("b.level")}
	registryMu.Unlock()
	items := []Metric{g("a.gone"), g("b.level"), c("z.count"), g("m.level"), c("y.gone")}
	SortMetrics("sorted", items, func(m Metric) Metric { return m })
	var got []string
	for _, m := range items {
		got = append(got, m.Name)
	}
	if want := []string{"z.count", "y.gone", "m.level", "b.level", "a.gone"}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}
