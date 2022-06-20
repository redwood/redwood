
# Demo: desktop chat app (embedded Redwood)

**NOTE: this demo is EXTREMELY WIP and is being actively developed!**

----

**Prerequisites**

- Go 1.18

**Running the demo**

In one terminal:

```sh
cd frontend && yarn && yarn build && cd ..
go run *.go --dev --config ./redwoodrc
```

A native desktop app should open. Windows will be supported shortly.

---

**Development workflow**

In one terminal:

```sh
go run *.go --dev --config ./redwoodrc
```

In a second terminal:

```sh
yarn start
```

Now, open a browser to http://localhost:3000 (`yarn start` should do this automatically).

