package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Retired means nothing about the bundle can change. The cascades read the bundle first and
// refuse before they compute or enqueue anything, so a person can't be given, or taken off, a
// bundle that is no longer given, and nothing is written.
func TestRetiredBundleRefusesEveryCascade(t *testing.T) {
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for name, run := range map[string]func(ctx context.Context) error{
		"assign": func(ctx context.Context) error {
			_, err := CascadeBundleAssignedToUser(ctx, "admin", "u1", "b1")
			return err
		},
		"remove": func(ctx context.Context) error {
			_, err := CascadeBundleRemovedFromUser(ctx, "admin", "u1", "b1")
			return err
		},
		"retire again": func(ctx context.Context) error {
			_, err := CascadeBundleRetired(ctx, "admin", "b1")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			resetCascadeDeps(t)
			svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
				return models.Bundle{ID: id, Name: "Old", RetiredAt: &at, RetiredBy: "admin"}, nil
			}
			wrote := false
			svcAssignBundleAndEnqueue = func(context.Context, string, string, string, string, []db.EnqueueParams) ([]string, bool, error) {
				wrote = true
				return nil, false, nil
			}
			svcRemoveBundleFromUserAndEnqueue = func(context.Context, string, string, string, []db.EnqueueParams) ([]string, error) {
				wrote = true
				return nil, nil
			}
			svcRetireBundleAndEnqueue = func(context.Context, string, string, []db.EnqueueParams) ([]string, error) {
				wrote = true
				return nil, nil
			}

			err := run(context.Background())
			var retired *ErrRetired
			if !errors.As(err, &retired) || !retired.At.Equal(at) {
				t.Fatalf("err = %v, want ErrRetired at %v", err, at)
			}
			if wrote {
				t.Fatal("a refused write must not reach the database")
			}
		})
	}
}

// A retired rule or bundle applies to nobody, so there is nothing for "what if it were gone" to
// ask: it answers as though it isn't there.
func TestWhatIf_RetiredBundleOrRuleIsNotFound(t *testing.T) {
	at := time.Now()
	for name, c := range map[string]models.InsightsWhatIfChange{
		"bundle": {Kind: "bundle_gone", BundleID: "b1"},
		"rule":   {Kind: "rule_gone", RuleID: "r1"},
	} {
		w := newWhatIfWorld("Amy")
		w.install(t)
		svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
			return models.Bundle{ID: id, RetiredAt: &at}, nil
		}
		svcGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
			return models.MappingRule{ID: id, RetiredAt: &at}, nil
		}
		if _, err := InsightsWhatIf(context.Background(), c); !errors.Is(err, ErrWhatIfNotFound) {
			t.Errorf("%s: err = %v, want ErrWhatIfNotFound", name, err)
		}
	}
}
