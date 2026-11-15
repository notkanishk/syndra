package addons

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Where a system's name came from, for the System page's Name setting.
const (
	NameFromOperator = "operator"
	NameFromProduct  = "product"
	NameFromID       = "id"
)

// SystemName is what a connected system is called (addon-contract 3.10a,
// design §6): the operator's name for it, else its accepted manifest's
// product, else its id title-cased; from says which. The one rule, so a page
// and a sentence the backend writes (an outbox row's reason) say one name.
// The caller reads the two values; this decides between them.
func SystemName(operator, product, id string) (name, from string) {
	if operator != "" {
		return operator, NameFromOperator
	}
	if strings.TrimSpace(product) != "" {
		return product, NameFromProduct
	}
	r, size := utf8.DecodeRuneInString(id)
	return string(unicode.ToUpper(r)) + id[size:], NameFromID
}
