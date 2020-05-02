
# Demo: collaborative text editor

You can run the text editor demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd demos/text-editor
$ go run main.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <https://localhost:21232>
- <https://localhost:21242>

**(Note: due to the self-signed TLS certificates, your browser will complain that you're "not safe")**

As with the chat room demo, you now have four nodes (two in Go, two in the browser) that can talk with one another.
