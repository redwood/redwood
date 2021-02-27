package main

import (
	"redwood.dev/remotestore"
	"redwood.dev/types"
)

func main() {
	addr1, err := types.AddressFromHex("96216849c49358b10257cb55b28ea603c874b05e")
	if err != nil {
		panic(err)
	}
	addr2, err := types.AddressFromHex("bd2eeb9c7dbe50264d528541c9d52142b616f55a")
	if err != nil {
		panic(err)
	}
	server := remotestore.NewServer("tcp", ":4567", "/tmp/badger-remote", []types.Address{addr1, addr2})

	err = server.Start()
	if err != nil {
		panic(err)
	}

	select {}
}
