
# Demo: desktop chat app (embedded Redwood)

**NOTE: this demo is EXTREMELY WIP and is being actively developed!**

----

**Prerequisites**

- Go 1.16
- Node.js 10+
- Ensure that you've run `yarn` inside the `frontend` directory.

**Running the demo**

In one terminal:

```sh
cd frontend && yarn && yarn build && cd ..
go run *.go --dev
```

In a second terminal:

```sh
node setup.js
```

---

**Working on the demo**

In one terminal:

```sh
go run *.go --dev
```

In a second terminal:

```sh
yarn start
```

In a third terminal:

```sh
node setup.js
```

Now, open a browser to http://localhost:3000 (`yarn start` should do this automatically).

