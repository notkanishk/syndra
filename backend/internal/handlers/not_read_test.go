package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
)

// A mapping or an allowance whose value could not be checked is not saved
// (design §13.3, owner Q2): one path, 503 TARGET_NOT_READ with the sentence.
func TestAnUncheckedValueIsNotSaved(t *testing.T) {
	orig := addonsResolvesValue
	t.Cleanup(func() { addonsResolvesValue = orig })
	addonsResolvesValue = func(context.Context, string, string, string) (addons.Resolution, error) {
		return addons.Resolution{}, addons.NotRead{Why: addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer}}
	}
	_, err := resolveValue(context.Background(), "truenas", "group", "lab_makers", nil)
	for name, write := range map[string]func(*httptest.ResponseRecorder){
		"mapping":   func(rr *httptest.ResponseRecorder) { writeMappingError(rr, err) },
		"allowance": func(rr *httptest.ResponseRecorder) { writeAllowanceError(rr, err) },
	} {
		rr := httptest.NewRecorder()
		write(rr)
		var body ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != 503 || body.Error != codeTargetNotRead || body.Details["cause"] != addons.CauseNoAnswer ||
			!strings.HasPrefix(body.Message, "Not saved. Syndra couldn't check this value on ") {
			t.Errorf("%s: %d %s", name, rr.Code, rr.Body)
		}
	}
}

// A system with no manifest is the same refusal, through the same path, for a
// mapping and an allowance alike: 503 with "Not saved…", never a 400.
func TestAValueOnASystemWithNoManifestIsNotSaved(t *testing.T) {
	orig := addonsEntitlementSchema
	t.Cleanup(func() { addonsEntitlementSchema = orig })
	addonsEntitlementSchema = func(string) ([]addons.EntitlementField, error) { return nil, addons.ErrNoManifest }
	_, _, _, mappingErr := validateMappingAgainstTarget(context.Background(), "truenas", "group", "lab_makers")
	_, _, allowanceErr := validateAllowanceAgainstTarget(context.Background(), "truenas", "group", "lab_makers")
	for name, write := range map[string]func(*httptest.ResponseRecorder){
		"mapping":   func(rr *httptest.ResponseRecorder) { writeMappingError(rr, mappingErr) },
		"allowance": func(rr *httptest.ResponseRecorder) { writeAllowanceError(rr, allowanceErr) },
	} {
		rr := httptest.NewRecorder()
		write(rr)
		var body ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != 503 || body.Error != codeTargetNotRead || !strings.HasPrefix(body.Message, "Not saved. Syndra couldn't check this value on ") {
			t.Errorf("%s: %d %s", name, rr.Code, rr.Body)
		}
	}
}

// The add-on's own sentence goes first only where it is the one that knows
// (§13.1): its app down, or its refusal. Elsewhere it is Syndra's.
func TestTheAddonsSentenceGoesFirstOnlyWhereItKnows(t *testing.T) {
	ctx := context.Background()
	for reason, first := range map[string]bool{
		addons.ReasonAppUnreachable: true, addons.ReasonReadRefused: true,
		addons.ReasonAddonUnreachable: false, addons.ReasonCooldown: false, addons.ReasonAddonOutdated: false,
	} {
		got := notReadSentence(ctx, "truenas", addons.Why{Reason: reason, Detail: "The add-on's own words."})
		if (got == "The add-on's own words.") != first {
			t.Errorf("%s: %q", reason, got)
		}
	}
}

// app_unreachable names the product from the accepted manifest, the one
// source a write's outcome names it from.
func TestTheAppIsNamedFromTheAcceptedManifest(t *testing.T) {
	orig, name := addonsAcceptedShape, dbTargetName
	t.Cleanup(func() { addonsAcceptedShape, dbTargetName = orig, name })
	dbTargetName = func(context.Context, string) (string, error) { return "Ring Vault", nil }
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", Product: "TrueNAS SCALE"}, nil
	}
	got := notReadSentence(context.Background(), "truenas", addons.Why{Reason: addons.ReasonAppUnreachable})
	if got != "Ring Vault's add-on is running but can't reach TrueNAS SCALE. Check that TrueNAS SCALE is on." {
		t.Errorf("%q", got)
	}
}

// A storage reply whose read is none is no copy: the card's storage is
// absent, never shown beside `read` none, whatever Storage() could decode.
// Here the reply's view is none (its schema is another shape's, which the
// body does not meet) while its storage decodes.
func TestStorageWithNoReadingIsAbsent(t *testing.T) {
	answerStorage(t, func(string, string) addons.StorageStatus {
		return addons.StorageStatus{Outcome: addons.OutcomeSucceeded, PlacesReadable: true}
	})
	served := addonsRead
	addonsRead = func(ctx context.Context, target, op, subject, actor string, args map[string]any, fresh bool) addons.ReadReply {
		reply := served(ctx, target, op, subject, actor, args, fresh)
		reply.Schema = "common/records.json"
		return reply
	}
	stubMyStorage(t, &storageHarness{entitled: true, bound: true})
	view, err := describeMyTarget(httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil), "truenas", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Read == nil || !view.Read.None() || view.Storage != nil || view.Standing != nil {
		t.Errorf("read %+v, storage %+v, standing %+v", view.Read, view.Storage, view.Standing)
	}
}
