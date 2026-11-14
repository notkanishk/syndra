package handlers

import (
	"testing"

	"syndra/internal/locale"
)

func TestHealthBodyCarriesLocaleOnlyWhenSet(t *testing.T) {
	old := locale.Tag
	t.Cleanup(func() { locale.Tag = old })

	locale.Tag = ""
	if _, ok := healthBody(map[string]string{"status": "ok"})["locale"]; ok {
		t.Error("locale sent although LOCALE is unset")
	}
	locale.Tag = "en-GB"
	if got := healthBody(map[string]string{"status": "ok"}); got["locale"] != "en-GB" || got["time_zone"] == "" {
		t.Errorf("body = %v; want locale en-GB beside time_zone", got)
	}
}
