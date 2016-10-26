package etp

import (
	"encoding/json"
	"testing"
)

type reqTest struct {
	Method     string
	RequestURI string
	Data       interface{}
	Result     string
}

var reqData = []reqTest{
	{
		"GET",
		"/zen",
		nil,
		"GET /zen ETP/1.0\r\n",
	},
	{
		"POST",
		"/auth/sign",
		map[string]string{"username": "ooclab", "password": "ooclab"},
		"POST /auth/sign ETP/1.0\r\nContent-Length: 41\r\n\r\n{\"password\":\"ooclab\",\"username\":\"ooclab\"}",
	},
}

func Test_NewRequest(t *testing.T) {
	// TODO: wanted 和 body 都使用 byte 测试为好, SerialDumps 可能会自行调整dict顺序
	for _, v := range reqData {
		var body []byte
		if v.Data != nil {
			body, _ = json.Marshal(v.Data)
		}
		req, err := NewRequest(v.Method, v.RequestURI, body)
		if err != nil {
			t.Error("NewRequest error:", err)
			continue
		}
		if string(req.Bytes()) != v.Result {
			t.Error("NewRequest error:", v.Method, v.RequestURI)
			continue
		}
	}
}

func Test_ReadRequest(t *testing.T) {
	for _, v := range reqData {
		req, err := ReadRequest([]byte(v.Result))
		if err != nil {
			t.Error("ReadRequest error:", err)
			continue
		}
		if string(req.Bytes()) != v.Result {
			t.Error("ReadRequest error:", v.Method, v.RequestURI)
			continue
		}
	}
}
