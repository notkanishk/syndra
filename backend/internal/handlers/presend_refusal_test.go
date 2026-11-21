package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"syndra/internal/services/addonop"
	"testing"

	"syndra/internal/addons"
)

// A write Syndra refused before sending is Syndra's own fault: a 500 with a
// reference under the write's own code, never a 502 blaming the add-on and
// never a physical action described as a credential. Parameters that do not
// fit are the caller's: 400.
func TestAPreSendRefusalIsSyndrasOwnAndNamedForItsWrite(t *testing.T) {
	own := errors.New("record not committed: pq: connection reset")
	bad := fmt.Errorf("%w: door.open: duration: too long", addons.ErrInvalidParams)
	for _, c := range []struct {
		name  string
		write func(http.ResponseWriter)
		code  int
		error string
	}{
		{"action", func(w http.ResponseWriter) { writeActionError(w, own) }, 500, "ACTION_NOT_SENT"},
		{"password", func(w http.ResponseWriter) { writeCredentialError(w, own) }, 500, "CREDENTIAL_NOT_SET"},
		{"operation", func(w http.ResponseWriter) { writeOperationError(w, own, "ADOPTION_FAILED", "") }, 500, "ADOPTION_FAILED"},
		{"operation params", func(w http.ResponseWriter) { writeOperationError(w, bad, "ADOPTION_FAILED", "") }, 400, "VALIDATION_FAILED"},
	} {
		rr := httptest.NewRecorder()
		c.write(rr)
		body := rr.Body.String()
		if rr.Code != c.code || !strings.Contains(body, `"error":"`+c.error+`"`) {
			t.Errorf("%s: got %d %s, want %d %s", c.name, rr.Code, body, c.code, c.error)
		}
		if strings.Contains(body, "connection reset") {
			t.Errorf("%s: Syndra's own error text leaked: %s", c.name, body)
		}
	}
}

// A member action refused for the caller's own request (unavailable, not
// confirmed, a resend, a read or a non-physical preview) is a 4xx with a true
// sentence, never Syndra's 500.
func TestAnActionRefusedForTheRequestIsTheCallers(t *testing.T) {
	for err, want := range map[error]int{
		&addons.ErrOperationUnavailable{Target: "truenas", ID: "door.open", Reason: "the door controller is offline"}: http.StatusConflict,
		fmt.Errorf("%w: truenas/door.open", addonop.ErrConfirmationRequired):                                          http.StatusUnprocessableEntity,
		fmt.Errorf("%w: truenas/door.open", addonop.ErrPhysicalNotResent):                                             http.StatusConflict,
		fmt.Errorf("%w: truenas/storage.get", addonop.ErrReadNotDispatched):                                           http.StatusBadRequest,
		fmt.Errorf("%w: truenas/password.set", addons.ErrNotPhysical):                                                 http.StatusBadRequest,
	} {
		rr := httptest.NewRecorder()
		writeActionError(rr, err)
		var out ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		if _, gone := err.(*addons.ErrOperationUnavailable); gone && out.Error != "ACTION_UNAVAILABLE" {
			t.Errorf("one code for an action that is not available, the one the member UI knows: %q", out.Error)
		}
		if rr.Code != want || !strings.Contains(out.Message, "Nothing was sent.") || strings.Contains(out.Message, "addonop") {
			t.Errorf("%v: got %d %q, want %d and a sentence", err, rr.Code, out.Message, want)
		}
	}
}
