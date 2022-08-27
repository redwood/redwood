package vault

//go:generate protoc -I=. -I=../ -I=$GOPATH/src --gogoslick_opt=paths=source_relative --gogoslick_out=plugins=grpc:. vault.proto
