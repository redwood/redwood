
# Demo: realtime collaborative text editor

**Prerequisites**

- Go 1.13
- Node.js 10+
- Ensure that you've run `npm i` inside the `braidjs` directory (located in the repository root)

**Running the demo**

First, compile `redwood.js`:

```sh
cd redwood.js
yarn && yarn build
```

Then, build the Redwood binary and place it in your `$PATH`:

```sh
cd ../cmd
go build --tags static -o /usr/local/bin/redwood .
```

Then, start the first Redwood node:

```sh
$ cd ../demos/text-editor
$ redwood --config ./node1.redwoodrc --password-file ./password.txt
```

Then, open another terminal and start the second Redwood node:

```sh
$ redwood --config ./node2.redwoodrc --password-file ./password.txt
```

Lastly, open another terminal and run the `setup.js` script:

```sh
node setup.js
```


Now, you can open two browser tabs to:
- <http://localhost:8080>
- <http://localhost:9090>

You now have four nodes (two in Go, two in the browser) that can talk with one another.  Try killing the Go nodes and continuing to send messages in the browser.  You'll notice that the browsers can still communicate (over WebRTC).
