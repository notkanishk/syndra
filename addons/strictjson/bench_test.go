package strictjson

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// planLike is a /plan request's shape: what the add-ons decode most.
type planLike struct {
	ContractVersion int `json:"contract_version"`
	Subjects        []struct {
		Subject string                     `json:"subject"`
		Email   string                     `json:"email"`
		Desired map[string]json.RawMessage `json:"desired"`
	} `json:"subjects"`
}

// planBody is a /plan of about 1 MB (constants.max_reply_bytes is the same
// order; a request is no larger in practice).
func planBody() []byte {
	var b strings.Builder
	b.WriteString(`{"contract_version":2,"subjects":[`)
	for i := 0; b.Len() < 1<<20; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"subject":"wwe-superstar-%d","email":"superstar-%d@example.org","desired":{"enabled":true,"group":["main","printers","after-hours"],"home":["true"],"home_quota":["5368709120"]}}`, i, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func BenchmarkDecodePlan1MB(b *testing.B) {
	body := planBody()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		var p planLike
		if err := Decode(body, &p); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodingJSONPlan1MB is the decode alone, for comparison.
func BenchmarkEncodingJSONPlan1MB(b *testing.B) {
	body := planBody()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	for b.Loop() {
		var p planLike
		if err := json.Unmarshal(body, &p); err != nil {
			b.Fatal(err)
		}
	}
}
