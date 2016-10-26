package etp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrNoHandler     = errors.New("no matched handler")
	ErrNoMethod      = errors.New("no matched method")
	ErrRequestError  = errors.New("convert msg to request failed")
	ErrContentLength = errors.New("Conn.Write wrote more than the declared Content-Length")
)

type Route struct {
	Method  string
	Pattern string
	Handler HandlerFunc
}

type Router struct {
	routes  []Route
	reMatch map[*regexp.Regexp]Route
}

func NewRouter() *Router {
	router := &Router{
		reMatch: make(map[*regexp.Regexp]Route),
	}
	return router
}

func (r *Router) AddRoute(route Route) {
	// Fix me
	pattern := route.Pattern
	if !strings.HasPrefix(route.Pattern, "^") {
		pattern = "^" + pattern
	}
	if !strings.HasSuffix(route.Pattern, "$") {
		pattern = pattern + "$"
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}
	r.reMatch[re] = route
	r.routes = append(r.routes, route)
}

func (r *Router) AddRoutes(routes []Route) {
	for _, v := range routes {
		r.AddRoute(v)
	}
}

func (r *Router) Dispatch(w ResponseWriter, req *Request) error {

	for re, v := range r.reMatch {
		matchs := re.FindStringSubmatch(req.URL.Path)
		if len(matchs) > 0 {
			if req.Method == v.Method {
				req.PathArgs = matchs[1:]
				v.Handler(w, req)
				return nil
			} else {
				w.WriteHeader(StatusMethodNotAllowed)
				return ErrNoMethod
			}
		}
	}

	w.WriteHeader(StatusNotFound)
	return ErrNoHandler
}

// 一个 ResponseWriter 实现
type response struct {
	wroteHeader bool
	header      http.Header

	written       int64
	contentLength int64
	status        int

	body []byte
	bb   *bytes.Buffer

	handlerDone bool // set true when the handler exits
}

func (w *response) Header() http.Header {
	return w.header
}

func (w *response) WriteHeader(code int) {
	if w.wroteHeader {
		log.Print("etp: multiple response.WriteHeader calls")
		return
	}
	w.wroteHeader = true
	w.status = code

	if cl := w.header.Get("Content-Length"); cl != "" {
		v, err := strconv.ParseInt(cl, 10, 64)
		if err == nil && v >= 0 {
			w.contentLength = v
		} else {
			log.Printf("etp: invalid Content-Length of %q", cl)
			w.header.Del("Content-Length")
		}
	}
}

func (w *response) Write(data []byte) (n int, err error) {
	return w.write(len(data), data, "")
}

func (w *response) WriteString(data string) (n int, err error) {
	return w.write(len(data), nil, data)
}

// either dataB or dataS is non-zero.
func (w *response) write(lenData int, dataB []byte, dataS string) (n int, err error) {
	if !w.wroteHeader {
		w.WriteHeader(StatusOK)
	}
	if lenData == 0 {
		return 0, nil
	}

	w.written += int64(lenData) // ignoring errors, for errorKludge
	if w.contentLength != -1 && w.written > w.contentLength {
		return 0, ErrContentLength
	}
	if dataB != nil {
		return w.bb.Write(dataB)
	} else {
		return io.WriteString(w.bb, dataS)
	}
}

func (w *response) Status() int {
	return w.status
}

func (w *response) Bytes() []byte {
	buf := new(bytes.Buffer)

	// init
	if w.status == 0 {
		w.status = 200
	}

	io.WriteString(buf, fmt.Sprintf("ETP/1.0 %d %s\r\n", w.status, StatusText(w.status)))

	if w.written > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(w.written, 10))
	}

	if w.Header() != nil {
		for k, v := range w.Header() {
			joinStr := ", "
			if k == "COOKIE" {
				joinStr = "; "
			}
			io.WriteString(buf, fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, joinStr)))
		}
	}
	if w.written > 0 {
		io.WriteString(buf, "\r\n")
		buf.Write(w.bb.Bytes())
	}

	return buf.Bytes()
}

func NewResponseWriter() ResponseWriter {
	return &response{
		header:        make(http.Header),
		bb:            new(bytes.Buffer),
		contentLength: -1, // TODO
	}
}

// Helper handlers

// Error replies to the request with the specified error message and HTTP code.
// The error message should be plain text.
func Error(w ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintln(w, error)
}

// NotFound replies to the request with an HTTP 404 not found error.
func NotFound(w ResponseWriter, r *Request) { Error(w, "404 page not found", StatusNotFound) }
