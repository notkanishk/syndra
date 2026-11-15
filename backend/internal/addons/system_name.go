package addons

import (
	"context"
	"log"
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

// SystemNameOf is what a connected system is called (addon-contract 3.10a,
// design §6): the operator's name for it, else its accepted manifest's product
// (saved at acceptance, so it holds while the add-on is down), else its id
// title-cased, the rule the UI's targetLabel draws a row with before any
// roster has loaded; from says which. The one path, so a page and a sentence
// the backend writes (an outbox row's reason) say one name.
func SystemNameOf(ctx context.Context, target string) (name, from string) {
	return SystemNameFrom(ctx, target, dbTargetName, AcceptedShape)
}

// SystemNameFrom is SystemNameOf over the caller's own reads of the two
// values, for a package that holds them as seams (handlers). A read that
// fails falls to the next value: a page that cannot read the name still has
// a true one.
func SystemNameFrom(ctx context.Context, target string,
	operatorName func(context.Context, string) (string, error),
	acceptedShape func(context.Context, string) (Shape, error)) (name, from string) {
	stored, err := operatorName(ctx, target)
	if err != nil {
		log.Printf("[TARGETS] could not read %s's name: %v (non-fatal)", target, err)
	}
	if stored != "" {
		return stored, NameFromOperator
	}
	if shape, err := acceptedShape(ctx, target); err == nil && strings.TrimSpace(shape.Product) != "" {
		return shape.Product, NameFromProduct
	}
	r, size := utf8.DecodeRuneInString(target)
	return string(unicode.ToUpper(r)) + target[size:], NameFromID
}
