package etp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/jmoiron/jsonq"
)

type Response struct {
	Status     string // e.g. "200 OK"
	StatusCode int    // e.g. 200

	Proto      string // "ETP/1.0"
	ProtoMajor int    // 1
	ProtoMinor int    // 0

	PathArgs  []string
	QueryArgs map[string][]string // TODO:

	Header http.Header
	Body   []byte

	ContentLength int64 // TODO: needed?
}

func (resp *Response) BodyQuery() *jsonq.JsonQuery {
	// TODO: 根据 Header["Content-Type"] 判断
	data := map[string]interface{}{}
	dec := json.NewDecoder(bytes.NewBuffer(resp.Body))
	dec.Decode(&data)
	return jsonq.NewQuery(data)
}

func (resp *Response) BodyLoads(obj interface{}) error {
	// TODO: 根据 Header["Content-Type"] 判断，处理不同类型
	dec := json.NewDecoder(bytes.NewBuffer(resp.Body))
	return dec.Decode(obj)
}

func (resp *Response) BodyError() string {
	q := resp.BodyQuery()
	error, err := q.String("error")
	if err != nil {
		fmt.Println("query error in body failed:", err)
	}
	return error
}

func ReadResponse(data []byte) (resp *Response, err error) {

	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(data)))
	resp = new(Response)

	// read first line: GET /index.html ETP/1.0
	line, err := tp.ReadLine()
	if err != nil {
		return nil, err
	}

	f := strings.SplitN(line, " ", 3)
	if len(f) < 2 {
		return nil, &badStringError{"malformed ETP response", line}
	}
	reasonPhrase := ""
	if len(f) > 2 {
		reasonPhrase = f[2]
	}
	resp.Status = f[1] + " " + reasonPhrase
	resp.StatusCode, err = strconv.Atoi(f[1])
	if err != nil {
		return nil, &badStringError{"malformed ETP status code", f[1]}
	}

	resp.Proto = f[0]
	var ok bool
	if resp.ProtoMajor, resp.ProtoMinor, ok = ParseHTTPVersion(resp.Proto); !ok {
		return nil, &badStringError{"malformed ETP version", resp.Proto}
	}

	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		// TODO: etp 定制? 无 headers
		if err == io.EOF {
			return resp, nil
		}
		return nil, err
	}
	resp.Header = http.Header(mimeHeader)

	if clen := resp.Header.Get("Content-Length"); len(clen) > 0 {
		resp.ContentLength, err = strconv.ParseInt(clen, 10, 64)
		if err != nil {
			fmt.Printf("parse Content-Length (%s): %s\n", clen, err)
			return nil, err
		}
	}

	// TODO: ugly?
	before := int64(len(data)) - resp.ContentLength
	resp.Body = data[before:]

	return resp, nil
}

// 参考 http.ResponseWriter
type ResponseWriter interface {
	Header() http.Header
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	WriteHeader(int)
	Bytes() []byte
	Status() int
}

// 一些实用工具

func WriteJSON(w ResponseWriter, code int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	if code > 0 {
		w.WriteHeader(code)
	}
	return json.NewEncoder(w).Encode(v)
}
