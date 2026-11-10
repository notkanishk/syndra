package reference

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The transport here is a copy; this holds the copy to the one file both real
// ends are held to. (Go's test cache does not see that file: -count=1.)
func TestTransportReproducesTheContractVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "contract", "transport_derivation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Secret      string `json:"secret_utf8"`
		Derivations []struct {
			Target  string `json:"target"`
			Seed    string `json:"tls_seed_hex"`
			Public  string `json:"tls_public_key_hex"`
			HMACKey string `json:"hmac_key_hex"`
		} `json:"derivations"`
		Raw struct {
			Vectors []struct {
				Name, Method, Target, Unix string
				PathAsSent                 string `json:"path_as_sent"`
				RawQuery                   string `json:"raw_query"`
				Body                       string `json:"body_utf8"`
				HMACKey                    string `json:"hmac_key_hex"`
				MAC                        string `json:"mac_hex"`
			} `json:"vectors"`
		} `json:"raw_query_signatures"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Derivations) < 2 || len(v.Raw.Vectors) < 4 {
		t.Fatalf("the vector file lost entries: %d derivations, %d raw-query vectors", len(v.Derivations), len(v.Raw.Vectors))
	}
	for _, d := range v.Derivations {
		priv, key, err := deriveKeys([]byte(v.Secret), d.Target)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(priv.Seed()); got != d.Seed {
			t.Errorf("%s: seed %s, want %s", d.Target, got, d.Seed)
		}
		if got := hex.EncodeToString(priv.Public().(ed25519.PublicKey)); got != d.Public {
			t.Errorf("%s: public key %s, want %s", d.Target, got, d.Public)
		}
		if got := hex.EncodeToString(key); got != d.HMACKey {
			t.Errorf("%s: HMAC key %s, want %s", d.Target, got, d.HMACKey)
		}
	}
	for _, vec := range v.Raw.Vectors {
		// The request is parsed as the server would parse it; the target the
		// MAC covers is taken from the parsed URL, which is what verify does.
		r := httptest.NewRequest(vec.Method, vec.Target, nil)
		if got := signedTarget(r.URL); got != vec.Target {
			t.Errorf("%s: signed target %q, want %q", vec.Name, got, vec.Target)
		}
		unix, _ := strconv.ParseInt(vec.Unix, 10, 64)
		key, _ := hex.DecodeString(vec.HMACKey)
		if got := hex.EncodeToString(computeMAC(unix, vec.Method, vec.Target, []byte(vec.Body), key)); got != vec.MAC {
			t.Errorf("%s: MAC %s, want %s", vec.Name, got, vec.MAC)
		}
		// And end to end through the authenticator, inside its window.
		a := authenticator{key: key, now: func() time.Time { return time.Unix(unix, 0) }}
		r = httptest.NewRequest(vec.Method, vec.Target, strings.NewReader(vec.Body))
		r.Header.Set(SignatureHeader, "t="+vec.Unix+",v1="+vec.MAC)
		if _, err := a.verify(r); err != nil {
			t.Errorf("%s: the authenticator refused the vector: %v", vec.Name, err)
		}
		if vec.RawQuery != "" {
			r = httptest.NewRequest(vec.Method, vec.PathAsSent, strings.NewReader(vec.Body))
			r.Header.Set(SignatureHeader, "t="+vec.Unix+",v1="+vec.MAC)
			if _, err := a.verify(r); err == nil {
				t.Errorf("%s: a MAC over the query verified a request without it", vec.Name)
			}
		}
	}
}

func TestParseDefectsRefusesAnUnknownName(t *testing.T) {
	if _, err := ParseDefects([]string{"ignores-everything"}); err == nil {
		t.Error("an unknown defect name was accepted")
	}
	set, err := ParseDefects(nil)
	if err != nil || len(set) != 0 {
		t.Errorf("no names gave %v, %v", set, err)
	}
}
