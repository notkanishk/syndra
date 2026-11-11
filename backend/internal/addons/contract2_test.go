package addons

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The raw-query vectors of contract 2 (addons/contract/transport_derivation.json),
// recomputed with the backend's own ComputeSignature. The checks over the
// shared contract files (schemas, registries, OpenAPI, JCS vectors) live once,
// in addons/truenas; contract_test.go and contract_routes_test.go hold this
// side's own messages to the fixtures, contract_schema_test.go to the schemas.

var v2Dir = filepath.Join("..", "..", "..", "addons", "contract", "v2")

// ---- the raw query joins the MAC (decision 24) ----------------------------

func v2Sign(unix, method, target string, body, key []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(unix + "." + method + "." + target + "."))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func TestV2RawQuerySignatureVectorsReproduce(t *testing.T) {
	var d struct {
		Derivations []struct {
			Target     string `json:"target"`
			HMACKeyHex string `json:"hmac_key_hex"`
		} `json:"derivations"`
		Signature struct {
			MACHex string `json:"mac_hex"`
		} `json:"signature"`
		Raw struct {
			Major   int `json:"introduced_by_major"`
			Vectors []struct {
				Name          string `json:"name"`
				Method        string `json:"method"`
				PathAsSent    string `json:"path_as_sent"`
				RawQuery      string `json:"raw_query"`
				Target        string `json:"target"`
				Body          string `json:"body_utf8"`
				Unix          string `json:"unix"`
				TargetName    string `json:"target_name"`
				KeyHex        string `json:"hmac_key_hex"`
				MACHex        string `json:"mac_hex"`
				DecodedMACHex string `json:"decoded_path_mac_hex"`
			} `json:"vectors"`
		} `json:"raw_query_signatures"`
	}
	raw, err := os.ReadFile(filepath.Join(v2Dir, "..", "transport_derivation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Raw.Major != 2 {
		t.Errorf("raw_query_signatures introduced_by_major = %d, want 2", d.Raw.Major)
	}
	if len(d.Raw.Vectors) != 4 {
		t.Fatalf("%d vectors, design §4 asks for four", len(d.Raw.Vectors))
	}
	derived := map[string]string{}
	for _, x := range d.Derivations {
		derived[x.Target] = x.HMACKeyHex
	}
	seen := map[string]bool{}
	for _, v := range d.Raw.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			seen[v.Name] = true
			want := v.PathAsSent
			if v.RawQuery != "" {
				want += "?" + v.RawQuery
			}
			if v.Target != want {
				t.Fatalf("target %q, want path as sent + ? + raw query = %q", v.Target, want)
			}
			if v.KeyHex != derived[v.TargetName] {
				t.Fatalf("key is not the one derived for target %q", v.TargetName)
			}
			key, _ := hex.DecodeString(v.KeyHex)
			if got := v2Sign(v.Unix, v.Method, v.Target, []byte(v.Body), key); got != v.MACHex {
				t.Errorf("MAC %s, vector says %s", got, v.MACHex)
			}
			if got := v2ProductionMAC(v.Unix, v.Method, v.Target, []byte(v.Body), key); got != v.MACHex {
				t.Errorf("the shipped signing function, given the target as its path, says %s; vector %s", got, v.MACHex)
			}
			// A query that is reordered or altered is a different request.
			if v.RawQuery != "" {
				for _, alt := range []string{v.RawQuery + "&x=1", reverseQuery(v.RawQuery)} {
					if alt != v.RawQuery && v2Sign(v.Unix, v.Method, v.PathAsSent+"?"+alt, []byte(v.Body), key) == v.MACHex {
						t.Errorf("query %q verifies as %q", alt, v.RawQuery)
					}
				}
			}
			switch v.Name {
			case "no_query":
				if v.MACHex != d.Signature.MACHex {
					t.Errorf("a query-less MAC must equal major 1's %s, got %s", d.Signature.MACHex, v.MACHex)
				}
			case "escaped_path":
				dec, err := url.PathUnescape(v.PathAsSent)
				if err != nil || dec == v.PathAsSent {
					t.Fatalf("the path does not need decoding: %q", v.PathAsSent)
				}
				decoded := v2Sign(v.Unix, v.Method, dec+"?"+v.RawQuery, []byte(v.Body), key)
				if decoded != v.DecodedMACHex || decoded == v.MACHex {
					t.Errorf("decoded-path MAC %s (vector %s, escaped %s): signing the decoded path must differ", decoded, v.DecodedMACHex, v.MACHex)
				}
			}
		})
	}
	for _, n := range []string{"no_query", "base64url_cursor", "reserved_and_repeated", "escaped_path"} {
		if !seen[n] {
			t.Errorf("vector %s is missing", n)
		}
	}
}

func reverseQuery(q string) string {
	parts := strings.Split(q, "&")
	slices.Reverse(parts)
	return strings.Join(parts, "&")
}

// v2ProductionMAC signs with the backend's own function, given the target as its
// path argument: both ends take the string to sign, so the major-2 change is in
// what the caller passes (slice 1), not in this function.
func v2ProductionMAC(unix, method, target string, body, key []byte) string {
	var u int64
	fmt.Sscan(unix, &u)
	h := ComputeSignature(time.Unix(u, 0), method, target, body, key)
	return h[strings.Index(h, "v1=")+3:]
}
