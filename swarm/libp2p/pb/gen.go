package pb

//go:generate protoc -I=. -I=../../.. -I=$GOPATH/src -I=$GOPATH/src/github.com/gogo/protobuf/protobuf --gogoslick_opt=paths=source_relative --gogoslick_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types:. libp2p.proto
