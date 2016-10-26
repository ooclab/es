package etp

import (
	"encoding/json"
	"testing"

	"github.com/jmoiron/jsonq"
)

type respTest struct {
	Status     string
	StatusCode int
	Data       interface{}
	Result     string
}

var respData = []respTest{
	{
		"200 OK",
		200,
		map[string]interface{}{
			"headers": map[string]interface{}{
				"Accept":     "*/*",
				"Host":       "httpbin.org",
				"User-Agent": "curl/7.35.0",
			},
		},
		`ETP/1.1 200 OK
Server: nginx
Date: Wed, 02 Sep 2015 15:08:51 GMT
Content-Type: application/json
Content-Length: 105
Connection: keep-alive
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true

{
  "headers": {
    "Accept": "*/*", 
    "Host": "httpbin.org", 
    "User-Agent": "curl/7.35.0"
  }
}`,
	},
}

func Test_ReadResponse(t *testing.T) {
	for _, v := range respData {
		resp, err := ReadResponse([]byte(v.Result))
		if err != nil {
			t.Error("ReadResponse error:", err)
			continue
		}

		var body map[string]interface{}
		err = json.Unmarshal(resp.Body, &body)
		if err != nil {
			t.Error("loads resp.Body failed:", err)
			continue
		}

		if resp.Status != v.Status ||
			resp.StatusCode != v.StatusCode {
			t.Error("ReadResponse failed")
			continue
		}

		q1 := jsonq.NewQuery(body)
		q2 := jsonq.NewQuery(v.Data.(map[string]interface{}))
		for _, key := range []string{"Accept", "Host", "User-Agent"} {
			v1, err := q1.String("headers", key)
			if err != nil {
				t.Errorf("q1 query %s failed: %s", key, err)
			}
			v2, _ := q2.String("headers", key)
			if err != nil {
				t.Errorf("q2 query %s failed: %s", key, err)
			}
			if v1 != v2 {
				t.Errorf("query %s failed: %s != %s", key, v1, v2)
			}
		}
	}
}
