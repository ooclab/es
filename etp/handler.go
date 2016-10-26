package etp

// 参考 net/http/server.go

type HandlerFunc func(ResponseWriter, *Request)

// ServeETP calls f(r)
func (f HandlerFunc) ServeETP(w ResponseWriter, r *Request) {
	f(w, r)
}
