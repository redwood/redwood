package pb

//go:generate protoc -I=. -I=$GOPATH/src -I=$GOPATH/src/github.com/gogo/protobuf/protobuf --gogoslick_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types:. tx.proto
