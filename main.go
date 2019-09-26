package main

import (
	"fmt"
)

func main() {
	store := NewStore()
	store.RegisterResolverForKeypath([]string{"foo", "bar"}, NewStaticResolver())
	store.RegisterResolverForKeypath([]string{}, NewDumbResolver())

	store.AddTx(Tx{
		Patches: []Patch{
			{
				Keys: []string{"foo"},
				Val:  123,
			},
		},
	})

	j, _ := store.StateJSON()
	fmt.Println(string(j), "\n")

	store.AddTx(Tx{
		Patches: []Patch{
			{
				Keys: []string{"foo", "bar", "baz"},
				Val:  123,
			},
		},
	})

	j, _ = store.StateJSON()
	fmt.Println(string(j), "\n")
}
