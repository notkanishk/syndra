package handlers

import (
	"context"
	"net/http"
	"os"

	"syndra/internal/claimdebug"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
)

// The claims target's route lives in claimserve (Door), so the claims
// container (cmd/claims, :8090) and this process's fallback route (:8080)
// run the same code (claims-delivery-integrity S7). These names keep the
// handlers package's vocabulary.

type (
	ActionV2UserRef        = claimserve.UserRef
	ActionV2UserGrantRef   = claimserve.GrantRef
	ActionV2ApplicationRef = claimserve.ApplicationRef
	ActionV2Request        = claimserve.Request
	ActionV2Claim          = claimserve.Claim
	ActionV2Response       = claimserve.Response
)

var knownTopLevel = claimserve.KnownTopLevel

func decodeActionV2(body []byte) (ActionV2Request, claimtel.Outcome) { return claimserve.Decode(body) }

// claimDoor is this process's door: the claimserve.Door the claims container
// serves, with this process's hooks (its recorder, its repair queue, its
// keyring reader, its debug mode). Each hook reads its package variable at
// call time, so a test that swaps one is heard.
var claimDoor = &claimserve.Door{
	Plane: func() *claimserve.Plane { return claimPlane },
	Keys:  actionSigningKeys,
	Refresh: func(ctx context.Context) []claimserve.SlotKey {
		claimPlane.RefreshKeys(ctx)
		return actionSigningKeys()
	},
	DevMode:    func() bool { return os.Getenv("ZITADEL_DOMAIN") == "" },
	Record:     func(c claimtel.Call) { recordClaimCall(c) },
	OnMiss:     func(u string) { claimRepair.enqueue(u) },
	OnMismatch: func() { nudgeKeyring() },
	Debug:      backendDebug{},
}

// HandleActionInject answers one Actions v2 call (claimserve.Door.Inject).
func HandleActionInject(w http.ResponseWriter, r *http.Request) { claimDoor.Inject(w, r) }

// backendDebug is debug mode in this process: the recorder explains each
// capture off the latency path.
type backendDebug struct{}

func (backendDebug) Wants(clientID, userID string) bool { return claimDebug.Wants(clientID, userID) }

func (backendDebug) Capture(c claimserve.DebugCapture) { claimDebug.Record(debugCapture(c)) }

func debugCapture(c claimserve.DebugCapture) claimdebug.Capture {
	return claimdebug.Capture{
		At: c.At, Function: c.Function, ClientID: c.ClientID, UserID: c.UserID, ProjectID: c.ProjectID,
		Grants: c.Grants, Request: c.Request, Response: c.Response, Duration: c.Duration,
	}
}
