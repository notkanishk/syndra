package main

// The `error` codes this add-on writes on the wire: the body of every non-2xx
// reply is `{"error": "<CODE>", ...extras}`.
//
// Named here, once, so the set the add-on can say is a list a test can compare
// against `addons/contract/registry/errors.json`. A code written inline at its
// call site is a code nothing checks: the registry would be a document, not a
// contract. The values are the wire and never change with the names.
const (
	CodeApplyFailed                  = "APPLY_FAILED"
	CodeBadRequest                   = "BAD_REQUEST"
	CodeCallIDReused                 = "CALL_ID_REUSED"
	CodeContractVersionMismatch      = "CONTRACT_VERSION_MISMATCH"
	CodeFingerprintRequired          = "FINGERPRINT_REQUIRED"
	CodeInvalidBody                  = "INVALID_BODY"
	CodeLifecycleRefusal             = "LIFECYCLE_REFUSAL"
	CodeNoCurrentRead                = "NO_CURRENT_READ"
	CodeNoSuchSetting                = "NO_SUCH_SETTING"
	CodeNotAHomePlace                = "NOT_A_HOME_PLACE"
	CodeOperationFailed              = "OPERATION_FAILED"
	CodeOperationMismatch            = "OPERATION_MISMATCH"
	CodePlanStale                    = "PLAN_STALE"
	CodeReachNeverRead               = "REACH_NEVER_READ"
	CodeReasonRequired               = "REASON_REQUIRED"
	CodeScopeAcknowledgementRequired = "SCOPE_ACKNOWLEDGEMENT_REQUIRED"
	CodeSettingNotSaved              = "SETTING_NOT_SAVED"
	CodeSettingsUnreadable           = "SETTINGS_UNREADABLE"
	CodeStoreUnreadable              = "STORE_UNREADABLE"
	CodeTargetUnreadable             = "TARGET_UNREADABLE"
	CodeTargetVersionUnsupported     = "TARGET_VERSION_UNSUPPORTED"
	CodeUnauthenticated              = "UNAUTHENTICATED"
	CodeUnknownField                 = "UNKNOWN_FIELD"
	CodeUnknownLifecycleState        = "UNKNOWN_LIFECYCLE_STATE"
)
