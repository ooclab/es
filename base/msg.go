package base

type Messager interface {
	String() string
	Dump() []byte
}
