package claims

import (
	"bytes"
	"strings"
)

// Change kinds. One Diff feeds the draft preview sentence, the before-line, the
// save summary, History's "What changed" and "Changed since the template".
const (
	ChangeAdd        = "add"         // a claim joins, or a role is now included (Role set)
	ChangeRemove     = "remove"      // a claim leaves, or a role is left out (Role set)
	ChangeRename     = "rename"      // a role's word changes (Role set)
	ChangeRetype     = "retype"      // same claim name, different kind or format
	ChangeMove       = "move"        // same claim content under a different name
	ChangeLands      = "lands"       // where the claims land
	ChangeIncludeNew = "include_new" // whether roles added later are included
)

// Change is one difference between two profiles. Role-level changes carry
// Role (the Syndra key) and the words in From/To; claim-level ones carry Path.
type Change struct {
	Kind    string   `json:"kind"`
	Role    string   `json:"role,omitempty"`
	Path    []string `json:"path,omitempty"`
	NewPath []string `json:"new_path,omitempty"` // move: where it went
	From    string   `json:"from,omitempty"`
	To      string   `json:"to,omitempty"`
}

// Diff lists what changes from old to new: claim rows in the new profile's
// order, then role words, then lands and roles-added-later.
func Diff(old, new Profile) []Change {
	old, new = Normalize(old), Normalize(new)
	out := []Change{}

	oldBy := map[string]Claim{}
	for _, c := range old.Claims {
		oldBy[pathKey(c.Path)] = c
	}
	newBy := map[string]Claim{}
	for _, c := range new.Claims {
		newBy[pathKey(c.Path)] = c
	}
	var gone []Claim
	for _, c := range old.Claims {
		if _, kept := newBy[pathKey(c.Path)]; !kept {
			gone = append(gone, c)
		}
	}
	for _, c := range new.Claims {
		prev, had := oldBy[pathKey(c.Path)]
		switch {
		case had && !sameContent(prev, c):
			out = append(out, Change{Kind: ChangeRetype, Path: c.Path, From: describe(prev), To: describe(c)})
		case !had:
			if i := indexSame(gone, c); i >= 0 {
				out = append(out, Change{Kind: ChangeMove, Path: gone[i].Path, NewPath: c.Path})
				gone = append(gone[:i], gone[i+1:]...)
				continue
			}
			out = append(out, Change{Kind: ChangeAdd, Path: c.Path, To: describe(c)})
		}
	}
	for _, c := range gone {
		out = append(out, Change{Kind: ChangeRemove, Path: c.Path, From: describe(c)})
	}

	// Role words matter only while both shapes send roles.
	if old.sendsRoles() && new.sendsRoles() {
		var keys []string
		seen := map[string]bool{}
		for _, m := range append(append([]RoleMap(nil), new.Roles...), old.Roles...) {
			if !seen[m.Key] {
				seen[m.Key] = true
				keys = append(keys, m.Key)
			}
		}
		for _, k := range keys {
			oi, ow := old.role(k)
			ni, nw := new.role(k)
			switch {
			case oi && ni && ow != nw:
				out = append(out, Change{Kind: ChangeRename, Role: k, From: ow, To: nw})
			case oi && !ni:
				out = append(out, Change{Kind: ChangeRemove, Role: k, From: ow})
			case !oi && ni:
				out = append(out, Change{Kind: ChangeAdd, Role: k, To: nw})
			}
		}
	}

	if old.Lands != new.Lands {
		out = append(out, Change{Kind: ChangeLands, From: old.Lands, To: new.Lands})
	}
	if old.IncludeNew != new.IncludeNew {
		out = append(out, Change{Kind: ChangeIncludeNew, From: yesNo(old.IncludeNew), To: yesNo(new.IncludeNew)})
	}
	return out
}

// ShapeOnly drops role-level changes. Which roles an app sees and their words
// are always the operator's, so they never count as "changed since the
// template".
func ShapeOnly(changes []Change) []Change {
	out := []Change{}
	for _, c := range changes {
		if c.Role == "" {
			out = append(out, c)
		}
	}
	return out
}

func (p Profile) sendsRoles() bool {
	for _, c := range p.Claims {
		if c.Kind == KindRoles || c.Kind == KindRoleEnds {
			return true
		}
	}
	return false
}

func pathKey(p []string) string { return strings.Join(p, "\x00") }

func sameContent(a, b Claim) bool {
	return a.Kind == b.Kind && a.Source == b.Source && a.Format == b.Format &&
		bytes.Equal(bytes.TrimSpace(a.Value), bytes.TrimSpace(b.Value)) &&
		strings.Join(a.Roles, "\x00") == strings.Join(b.Roles, "\x00")
}

func indexSame(cs []Claim, c Claim) int {
	for i, o := range cs {
		if sameContent(o, c) {
			return i
		}
	}
	return -1
}

// describe is a claim's content in one token: "roles:array", "attribute:email",
// "static:\"x\"", "if_role", "bundles", "role_ends".
func describe(c Claim) string {
	switch c.Kind {
	case KindRoles:
		return c.Kind + ":" + c.Format
	case KindAttribute:
		return c.Kind + ":" + c.Source
	case KindStatic:
		return c.Kind + ":" + string(bytes.TrimSpace(c.Value))
	default:
		return c.Kind
	}
}

func yesNo(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
