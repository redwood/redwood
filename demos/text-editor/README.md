
# Demo: collaborative text editor

You can run the text editor demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd demos/text-editor
$ go run --tags static main.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <http://localhost:21232>
- <http://localhost:21242>

As with the chat room demo, you now have four nodes (two in Go, two in the browser) that can talk with one another.
