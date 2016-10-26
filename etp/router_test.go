package etp

import (
	"fmt"
	"testing"
)

var routes = []Route{
	{"GET", "/zen", getZen},
	{"POST", `/nodes/([\-0-9a-fA-F]+)`, updateNode},
}

func getZen(w ResponseWriter, req *Request) {
	w.WriteHeader(StatusOK)
	w.Write([]byte("KISS"))
}

func updateNode(w ResponseWriter, req *Request) {
	w.WriteHeader(StatusOK)
	w.Header().Set("test_key", "test_value")
	w.WriteString("success!")
}

func Test_Dispatch(t *testing.T) {
	router := NewRouter()
	router.AddRoutes(routes)

	for _, v := range [][]string{
		{"GET /zen ETP/1.0\r\n", "ETP/1.0 200 OK\r\n\r\nKISS"},
		{"POST /nodes/ea64525d-fe0c-44a0-a1c6-2bc579e5b72d ETP/1.0\r\n\r\nContent-Length: 41\r\n\r\n{\"password\":\"ooclab\",\"username\":\"ooclab\"}", "ETP/1.0 200 OK\r\ntest_key: test_value\r\n\r\nsuccess!"},
	} {
		w := NewResponseWriter()
		req, err := ReadRequest([]byte(v[0]))
		if err != nil {
			t.Errorf("bad request:", err)
			continue
		}
		err = router.Dispatch(w, req)
		if err != nil {
			t.Errorf("handle error: %s\n%s", err, v[0])
			continue
		}
		if string(w.Bytes()) != v[1] {
			fmt.Println("missmatch resp_data : ", string(w.Bytes()))
		}
	}
}
