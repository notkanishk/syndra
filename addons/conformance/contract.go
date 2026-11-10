package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Major is the contract major this kit checks. It refuses a contract directory
// of any other major rather than checking an add-on against the wrong files.
const Major = 2

const schemaBase = "https://syndra.example.org/addon-contract/2/schema/"

// contract is addons/contract as the kit was built with it (embedded, so the
// binary carries the contract it checks against): the v2 schemas compiled with
// format assertion on, the registries, and the transport vector. Nothing is
// fetched; a $ref to anything not embedded fails to compile.
type contract struct {
	compiler *jsonschema.Compiler
	schemas  map[string]*jsonschema.Schema

	errors map[string]struct {
		Title    string `json:"title"`
		Statuses []int  `json:"statuses"`
		Retry    string `json:"retry"`
	}
	operations map[string]struct {
		Params []struct {
			Name     string `json:"name"`
			Required bool   `json:"required"`
		} `json:"params"`
		SecretParams []string `json:"secret_params"`
	}
	features map[string]struct {
		RequiresKind string `json:"requires_kind"`
	}
	kinds map[string]struct {
		ScopeDenylist []string `json:"scope_denylist"`
	}
	maxReplyBytes int64
	vectors       []byte // transport_derivation.json, raw
}

type offline struct{}

func (offline) Load(url string) (any, error) {
	return nil, fmt.Errorf("%s is not in the embedded contract, and the kit fetches nothing", url)
}

// loadContract reads v2/ and transport_derivation.json from files.
func loadContract(files fs.FS) (*contract, error) {
	const v2 = "v2"
	c := &contract{compiler: jsonschema.NewCompiler(), schemas: map[string]*jsonschema.Schema{}}
	c.compiler.AssertFormat()
	c.compiler.UseLoader(offline{})
	err := fs.WalkDir(files, v2+"/schema", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".json" {
			return err
		}
		raw, err := fs.ReadFile(files, p)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		id, _ := doc.(map[string]any)["$id"].(string)
		return c.compiler.AddResource(id, doc)
	})
	if err != nil {
		return nil, fmt.Errorf("load the embedded schemas: %w", err)
	}
	manifest, err := c.schema("manifest.json")
	if err != nil {
		return nil, err
	}
	if v, ok := manifest.Properties["contract_version"]; !ok || v.Const == nil || fmt.Sprint(*v.Const) != fmt.Sprint(Major) {
		return nil, fmt.Errorf("%s is not contract major %d", v2, Major)
	}

	var constants struct {
		Constants struct {
			MaxReplyBytes struct {
				Value int64 `json:"value"`
			} `json:"max_reply_bytes"`
		} `json:"constants"`
	}
	for file, into := range map[string]any{
		"errors.json":     &struct{ Codes any }{&c.errors},
		"operations.json": &struct{ Operations any }{&c.operations},
		"features.json":   &struct{ Features any }{&c.features},
		"kinds.json":      &struct{ Kinds any }{&c.kinds},
		"constants.json":  &constants,
	} {
		raw, err := fs.ReadFile(files, v2+"/registry/"+file)
		if err == nil {
			err = json.Unmarshal(raw, into)
		}
		if err != nil {
			return nil, fmt.Errorf("registry %s: %w", file, err)
		}
	}
	if c.maxReplyBytes = constants.Constants.MaxReplyBytes.Value; c.maxReplyBytes <= 0 || len(c.errors) == 0 || len(c.kinds) == 0 {
		return nil, fmt.Errorf("the registries under %s are incomplete", v2)
	}
	if c.vectors, err = fs.ReadFile(files, "transport_derivation.json"); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *contract) schema(name string) (*jsonschema.Schema, error) {
	if s, ok := c.schemas[name]; ok {
		return s, nil
	}
	s, err := c.compiler.Compile(schemaBase + name)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", name, err)
	}
	c.schemas[name] = s
	return s, nil
}

// validate checks a decoded JSON value (numbers as json.Number) against a
// schema under v2/schema, and says why it does not validate in one line.
func (c *contract) validate(name string, doc any) error {
	s, err := c.schema(name)
	if err != nil {
		return err
	}
	if err := s.Validate(doc); err != nil {
		return fmt.Errorf("does not validate against %s: %s", name, oneLine(err.Error()))
	}
	return nil
}

func oneLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 1 {
		lines = lines[1:] // the first line only names the schema
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "; ")
}
