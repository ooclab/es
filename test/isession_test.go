package test

import (
	"crypto/rand"
	"testing"

	"github.com/Sirupsen/logrus"

	"github.com/ooclab/es/isession"
)

func testEq(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func Benchmark_LinkInnerSessionSingle(b *testing.B) {
	_, clientLink, _ := getServerAndClient()

	s, _ := clientLink.OpenInnerSession()
	for i := 0; i < b.N; i++ { //use b.N for looping
		s.SendAndWait(&isession.Request{Action: "/echo", Body: []byte("Ping")})
	}
}

func Benchmark_LinkInnerSessionMulti(b *testing.B) {
	_, clientLink, _ := getServerAndClient()

	for i := 0; i < b.N; i++ { //use b.N for looping
		s, _ := clientLink.OpenInnerSession()
		s.SendAndWait(&isession.Request{Action: "/echo", Body: []byte("Ping")})
	}
}

func Test_LinkInnerSessionMinimalFrame(t *testing.T) {
	_, clientLink, _ := getServerAndClient()

	s, _ := clientLink.OpenInnerSession()
	for i := 0; i < 1025; i++ {
		if !testLinkInnerSession(s, i) {
			t.Error("response and request mismatch!")
			return
		}
	}
}

// FIXME! remote endpoint would close connection when send large message, test it someday!
// func Test_LinkInnerSessionLargeFrame(t *testing.T) {
// 	port, _ := startServer()
// 	l := connectServer(fmt.Sprintf("127.0.0.1:%d", port))
// 	defer l.Close()
//
// 	s, _ := l.OpenInnerSession()
// 	// FIXME: session request package is large than raw payload
// 	for _, i := range []int{4194303, 4194304, 4194305} {
// 		if !testLinkInnerSession(s, i) {
// 			t.Error("response and request mismatch!")
// 			return
// 		}
// 	}
// }

func testLinkInnerSession(s *isession.Session, length int) bool {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		logrus.Errorf("read rand failed: %s", err)
		return false
	}

	resp, err := s.SendAndWait(&isession.Request{Action: "/echo", Body: b})
	if err != nil {
		logrus.Error("request failed:", resp, err, length)
		return false
	}
	return testEq(b, resp.Body)
}
