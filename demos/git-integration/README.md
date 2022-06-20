
# Demo: Git integration

**Prerequisites**

- Go 1.18
- Node.js 10+

Initialize the embedded copy of `redwood.js`:
- Navigate to the `embed` folder in the project root
- Run `npm install` or `yarn`

Build the Git remote helper and install it into your `$PATH` (feel free to choose a different location, as long as it's in `$PATH`):
```sh
cd ../../cmd/git-remote-helper
go build --tags static -o /usr/local/bin/git-remote-redwood .
```

Build the Redwood binary:
```sh
cd ../cmd/redwood
go build .
```

**Running the demo**

Start the first Redwood node:

```sh
cd ../demos/git-integration
redwood --config ./node1.redwoodrc --password-file ./password.txt
```

Then, open another terminal and start the second Redwood node:

```sh
redwood --config ./node2.redwoodrc --password-file ./password.txt
```

Lastly, open another terminal and run the `setup.js` script:

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
- <http://localhost:8080/demo>
- <http://localhost:9090/demo>


Now, let's clone the demo repo.  Notice in the command below that the address of the local node is specified before the `@` character, and the State-URI is specified after.  If the Redwood node you were cloning from were actually hosted at `somegitprovider.org/gitdemo` rather than our small local environment, the leading portion of the URL could be omitted.

```sh
cd /tmp
git clone redwood://localhost:8080@somegitprovider.org/gitdemo
```

You'll notice that the file tree is identical to the one shown in your browser.  Also, notice that you can navigate to these files directly in the web browser -- the contents of the Git repo are served directly by Redwood (there are ways to handle staging/dev setups, but they're not covered by the demo).

Let's try pushing some commits.

- Try editing README.md, committing, and pushing.  You'll see that the browser updates the contents of the file instantly without a page refresh.
- Try copying a new JPEG image over `redwood.jpg`.  Commit and push.  The image will instantly update in the browser.
- Try editing `script.js` so that the function defined in that file returns `i + 100`.  Commit and push.  The script's behavior will immediately update to reflect your change, and will start counting up by 100.

If you push updates to `index.html`, you'll need to refresh the demo page in the browser to see your changes (but they will still have occurred instantly).

