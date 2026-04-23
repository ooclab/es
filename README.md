# es
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/ooclab/es)
[![Go Report Card](https://goreportcard.com/badge/github.com/labstack/echo?style=flat-square)](https://goreportcard.com/report/github.com/ooclab/es)
[![Build Status](http://img.shields.io/travis/ooclab/es.svg?style=flat-square)](https://travis-ci.org/ooclab/es)


Easy session/stream protocol

This is v2 of emsg (our private Easy Message Protocol)

OLD emsg and otunnel usage, ref to [Chinese usage of otunnel](http://ooclab.github.io/)

## Arch

### Message split design

simple message flow in one order

![](./docs/common/msg_split_design.png)

## Example

- [Simple Example](./example)
- [otunnel](https://github.com/ooclab/otunnel)

## CI

GitHub Actions workflow: `.github/workflows/ci.yml`.
It validates module metadata and runs `go build ./...` plus `go test ./...` on push/PR.
