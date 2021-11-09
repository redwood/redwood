package pb

//go:generate protoc -I=. -I=../.. -I=$GOPATH/src --gogoslick_opt=paths=source_relative --gogoslick_out=github.com/gogo/protobuf/types:. state.proto
