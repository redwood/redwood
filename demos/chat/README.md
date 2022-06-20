
# Demo: chat room

**Prerequisites**

- Go 1.18
- Node.js 10+

Initialize the embedded copy of `redwood.js`:
- Navigate to the `embed` folder in the project root
- Run `npm install` or `yarn`

Build the Redwood binary and place it in your `$PATH`:
```sh
cd ../../cmd/redwood
go build .
```

**Running the demo**

Start the first Redwood node:

```sh
cd ../demos/chat
redwood --config ./node1.redwoodrc --password-file ./password.txt
```

Then, open another terminal and start the second Redwood node:

```sh
redwood --config ./node2.redwoodrc --password-file ./password.txt
```

Lastly, open another terminal and run the `setup.js` script.  

```sh
# setup.js needs the @redwood.dev/client library
yarn

node setup.js
```

**NOTE:** if you receive an error about "digital envelope routines" failing to initialize, your version of Node.js requires you to specify an extra environment variable to function properly:

```sh
NODE_OPTIONS=--openssl-legacy-provider node setup.js
```


Now, you can open two browser tabs to:
- <http://localhost:8080>
- <http://localhost:9090>

