
# Demo: chat room

You can run the chat demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd demos/chat
$ go run --tags static main.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <http://localhost:21232/talk0>
- <http://localhost:21242/talk0>

You now have four nodes (two in Go, two in the browser) that can talk with one another.
