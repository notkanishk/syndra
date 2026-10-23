package directory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func TestToUserProfile_CarriesKind(t *testing.T) {
	if got := toUserProfile(zitadel.ZitadelUser{ID: "u1"}); got.Kind != models.UserKindHuman {
		t.Errorf("kind = %q; want human", got.Kind)
	}
	got := toUserProfile(zitadel.ZitadelUser{ID: "m1", Username: "netbird", Machine: true})
	if got.Kind != models.UserKindMachine || !got.IsMachine() {
		t.Errorf("kind = %q; want machine", got.Kind)
	}
	raw, _ := json.Marshal(got)
	if !strings.Contains(string(raw), `"kind":"machine"`) {
		t.Errorf("json %s must carry kind", raw)
	}
}

func TestDemoSource_EveryoneIsHuman(t *testing.T) {
	users, err := NewDemoSource().Users(context.Background())
	if err != nil || len(users) == 0 {
		t.Fatalf("demo users: %v %d", err, len(users))
	}
	for _, u := range users {
		if u.Kind != models.UserKindHuman {
			t.Errorf("%s kind = %q; want human", u.ID, u.Kind)
		}
	}
}
