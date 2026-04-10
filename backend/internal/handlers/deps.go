package handlers

import (
	"mkauth/internal/cache"
	"mkauth/internal/db"
)

var (
	dbUpsertDirectGrant = db.UpsertDirectGrant
	dbGetAccessRequests = db.GetAccessRequests
	dbCreateAccessRequest = db.CreateAccessRequest
	dbGetAccessRequestByID = db.GetAccessRequestByID
	dbResolveAccessRequest = db.ResolveAccessRequest
	dbInsertAuditLog = db.InsertAuditLog

	cacheRebuildUser = cache.RebuildUserCache
)
