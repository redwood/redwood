
# Demo: Git integration

Before running the Git demo, you'll need to build the Git remote helper and install it into your `$PATH` (feel free to choose a different location, as long as it's in `$PATH`):

```sh
$ cd git-remote-helper
$ go build --tags static -o /usr/bin/git-remote-redwood .
```

Once the helper is installed, you can run the git demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd demos/git-integration
$ go run --tags static main.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <http://localhost:21232/demo>
- <http://localhost:21242/demo>

Now, let's clone the demo repo.  Notice in the command below that the address of the local node is specified before the `@` character, and the State-URI is specified after.  If the Redwood node you were cloning from were actually hosted at `somegitprovider.org/gitdemo` rather than our small local environment, this portion of the URL could be omitted.

```sh
$ cd /tmp
$ git clone redwood://localhost:21231@somegitprovider.org/gitdemo
```

You'll notice that the file tree is identical to the one shown in your browser.  Also, notice that you can navigate to these files directly in the web browser -- the contents of the Git repo are served directly by Redwood (there are ways to handle staging/dev setups, but they're not covered by the demo).

Let's try pushing some commits.

- Try editing README.md, committing, and pushing.  You'll see that the browser updates the contents of the file instantly.
- Try copying a new JPEG image over `redwood.jpg`.  Commit and push.  The image will instantly update in the browser.
- Try editing `script.js` so that the function defined in that file returns `i + 100`.  Commit and push.  The script's behavior will immediately update to reflect your change, and will start counting up by 100.

If you push updates to `index.html`, you'll need to refresh the demo page in the browser to see your changes (but they will still have occurred instantly).

