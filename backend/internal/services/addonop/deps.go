package addonop

import (
	"syndra/internal/addons"
	"syndra/internal/db"
)

// Injectable dependencies, matching the pattern used across the backend
// (services/deps.go, cache/deps.go, addons/deps.go). The dispatch protocol's
// whole content is an ORDER of operations, and an order can only be tested by
// observing when each step happens — which needs seams, not a database.
var (
	resolveOperation = addons.ResolveOperation
	validateParams   = addons.ValidateParams
	operationRecord  = addons.OperationRecord
	callAddon        = addons.Call
	previewAddon     = addons.Preview
	beginOperation   = db.BeginAddonOperation
	// The rate limit counts the record table itself, so the thing bounded and
	// the thing counted cannot drift apart.
	countRecentOperations = db.CountRecentAddonOperations
	settleOperation       = db.SettleAddonOperation
	// A physical operation is refused while the subject's earlier one is
	// unsettled: the same read the member's page shows it from.
	unconfirmedChangeAt = db.UnconfirmedChangeAt
)
