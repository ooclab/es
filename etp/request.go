package etp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/jmoiron/jsonq"
)

type Request struct {
	Method string
	URL    *url.URL

	Proto      string // "ETP/1.0"
	ProtoMajor int    // 1
	ProtoMinor int    // 0

	PathArgs  []string
	QueryArgs map[string][]string // TODO:
	ExtHeader http.Header         // 扩展头部

	Header http.Header
	Body   []byte

	ContentLength int64 // TODO: needed?

	RequestURI string
}

func NewRequest(method, urlStr string, body []byte) (req *Request, err error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	req = &Request{
		Method:     method,
		URL:        u,
		Proto:      "ETP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       body,

		ExtHeader: make(http.Header),
	}
	req.ContentLength = int64(len(body))
	if req.ContentLength > 0 {
		req.Header.Set("Content-Length", strconv.FormatInt(req.ContentLength, 10))
	}

	return req, nil
}

func (req *Request) Query() *jsonq.JsonQuery {
	data := map[string]interface{}{}

	// url
	for k, v := range req.URL.Query() {
		switch len(v) {
		case 0:
			continue
		case 1:
			data[k] = v[0]
		default:
			data[k] = v
		}
	}

	// body
	obj := req.getBodyObj()
	if obj != nil {
		for k, v := range obj {
			data[k] = v
		}
	}

	return jsonq.NewQuery(data)
}

func (req *Request) getBodyObj() map[string]interface{} {
	t := req.Header.Get("Content-Type")
	if strings.Contains(t, "application/json") {
		data := map[string]interface{}{}
		dec := json.NewDecoder(bytes.NewBuffer(req.Body))
		dec.Decode(&data)
		return data
	}
	return nil
}

func (req *Request) BodyQuery() *jsonq.JsonQuery {
	// TODO: 根据 Header["Content-Type"] 判断
	data := map[string]interface{}{}
	dec := json.NewDecoder(bytes.NewBuffer(req.Body))
	dec.Decode(&data)
	return jsonq.NewQuery(data)
}

func (req *Request) Bytes() []byte {
	buf := new(bytes.Buffer)

	io.WriteString(buf, fmt.Sprintf("%s %s ETP/1.0\r\n", req.Method, req.URL.RequestURI()))
	if req.Header != nil {
		for k, v := range req.Header {
			joinStr := ", "
			if k == "COOKIE" {
				joinStr = "; "
			}
			io.WriteString(buf, fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, joinStr)))
		}
	}
	if req.Body != nil {
		//io.WriteString(buf, fmt.Sprintf("Content-Length: %d\r\n", len(req.Body)))
		io.WriteString(buf, "\r\n")
		buf.Write(req.Body)
	}

	return buf.Bytes()
}

func ReadRequest(data []byte) (req *Request, err error) {

	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(data)))
	req = new(Request)
	// TODO: 初始化
	req.Header = make(http.Header)
	req.ExtHeader = make(http.Header)

	// read first line: GET /index.html ETP/1.0
	var s string
	if s, err = tp.ReadLine(); err != nil {
		return nil, err
	}

	var ok bool
	req.Method, req.RequestURI, req.Proto, ok = parseRequestLine(s)
	if !ok {
		return nil, &badStringError{"malformed ETP request", s}
	}

	if req.URL, err = url.ParseRequestURI(req.RequestURI); err != nil {
		return nil, err
	}

	mimeHeader, err := tp.ReadMIMEHeader()
	// TODO: Important! Add First!
	if mimeHeader != nil {
		req.Header = http.Header(mimeHeader)
	}
	if err != nil {
		// TODO: etp 定制? 无 headers
		if err == io.EOF {
			return req, nil
		}
		return nil, err
	}

	if clen := req.Header.Get("Content-Length"); len(clen) > 0 {
		req.ContentLength, err = strconv.ParseInt(clen, 10, 64)
		if err != nil {
			fmt.Printf("parse Content-Length (%s): %s\n", clen, err)
			return nil, err
		}
	}

	// TODO: ugly?
	before := int64(len(data)) - req.ContentLength
	req.Body = data[before:]

	return req, nil
}

// parseRequestLine parses "GET /foo ETP/1.0" into its three parts.
func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}
