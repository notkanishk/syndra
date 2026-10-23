package zitadel

import (
	"context"
	"net/http"
	"testing"
)

// Zitadel's user search gives each user `human` or `machine`. The read only
// takes one more field from the same call.
func TestListUsers_ReadsUserType(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		return rawResp(200, `{"details":{"totalResult":"3"},"result":[
			{"id":"u1","userName":"a","state":"USER_STATE_ACTIVE","human":{"email":{"email":"a@example.edu"}}},
			{"id":"m1","userName":"netbird","state":"USER_STATE_ACTIVE","machine":{"name":"netbird","accessTokenType":"ACCESS_TOKEN_TYPE_JWT"}},
			{"id":"m2","userName":"bare","state":"USER_STATE_ACTIVE","machine":{}}]}`), nil
	}
	res, err := client.ListUsers(context.Background(), SearchParams{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Items[0].Machine || !res.Items[1].Machine || !res.Items[2].Machine {
		t.Errorf("machine flags = %v %v %v; want false true true", res.Items[0].Machine, res.Items[1].Machine, res.Items[2].Machine)
	}
}
