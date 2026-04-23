package logger

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// Fields is a set of key/value pairs attached to a log message.
type Fields map[string]interface{}

// Entry is a logger with bound fields.
type Entry struct {
	fields Fields
}

var std = log.New(os.Stderr, "", log.LstdFlags)

// Level keeps compatibility with existing SetLevel call sites.
type Level uint32

const (
	DebugLevel Level = iota
)

func SetLevel(_ Level) {}

func Debug(args ...interface{})                 { std.Print(args...) }
func Debugf(format string, args ...interface{}) { std.Printf(format, args...) }
func Infof(format string, args ...interface{})  { std.Printf(format, args...) }
func Warn(args ...interface{})                  { std.Print(args...) }
func Warnf(format string, args ...interface{})  { std.Printf(format, args...) }
func Error(args ...interface{})                 { std.Print(args...) }
func Errorf(format string, args ...interface{}) { std.Printf(format, args...) }
func Fatalln(args ...interface{})               { std.Fatalln(args...) }

func WithField(key string, value interface{}) *Entry {
	return &Entry{fields: Fields{key: value}}
}

func WithFields(fields Fields) *Entry {
	cp := make(Fields, len(fields))
	for k, v := range fields {
		cp[k] = v
	}
	return &Entry{fields: cp}
}

func (e *Entry) WithField(key string, value interface{}) *Entry {
	fields := e.copyFields()
	fields[key] = value
	return &Entry{fields: fields}
}

func (e *Entry) WithFields(extra Fields) *Entry {
	fields := e.copyFields()
	for k, v := range extra {
		fields[k] = v
	}
	return &Entry{fields: fields}
}

func (e *Entry) Debug(args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprint(args...))
}

func (e *Entry) Debugf(format string, args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprintf(format, args...))
}

func (e *Entry) Warn(args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprint(args...))
}

func (e *Entry) Error(args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprint(args...))
}

func (e *Entry) Errorf(format string, args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprintf(format, args...))
}

func (e *Entry) Warnf(format string, args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprintf(format, args...))
}

func (e *Entry) Infof(format string, args ...interface{}) {
	std.Print(e.prefix() + fmt.Sprintf(format, args...))
}

func (e *Entry) copyFields() Fields {
	cp := make(Fields, len(e.fields))
	for k, v := range e.fields {
		cp[k] = v
	}
	return cp
}

func (e *Entry) prefix() string {
	if len(e.fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(e.fields))
	for k := range e.fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, e.fields[k]))
	}
	return "[" + strings.Join(parts, " ") + "] "
}
