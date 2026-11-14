// Package contract is the add-on wire contract as files, for a binary that
// must carry the contract it was built from (the conformance kit, design §7).
//
// Files only: no Go type, constant or helper lives in this package, and none
// ever may. Design §1 rejects a shared Go module of types (a contract only for
// Go, and a replace directive that hides version skew); the two ends of the
// wire keep their own hand-written types and are held to these files by their
// own test suites, reading them from disk. Only the kit imports this package.
// The one Go code in the module is its subpackage strictjson: no contract
// type or fact, but the request decoding RFC 8259 §4 and RFC 7493 §2.3 ask
// of every add-on, shared by the two Go add-ons (design §1, slice 7o).
package contract

import (
	"embed"
	"io/fs"
)

//go:embed v2 transport_derivation.json
var files embed.FS

// FS is the current major's files (v2/...) and transport_derivation.json,
// at the same paths as in this directory.
func FS() fs.FS { return files }
