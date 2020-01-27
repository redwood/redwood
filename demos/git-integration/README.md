
# Demo: Git integration

Before running the Git demo, you'll need to build the Git remote helper and install it into your `$PATH`:

```sh
$ cd git-remote-helper
$ go build --tags static -o /usr/bin/git-remote-redwood main.go
```

Once the helper is installed, you can run the git demo with the following commands (note: Go 1.13+ is required).

```sh
$ cd demos/git-integration
$ go run main.go
```

This will spin up two nodes in the same process.  Once they're up, open browser tabs to:
- <https://localhost:21232/gitdemo/index.html>
- <https://localhost:21242/gitdemo/index.html>

Now, let's clone the demo repo:

```sh
$ git clone redwood://localhost:21231/git /tmp/redwood-git
$ cd /tmp/redwood-git
```

You'll notice that the file tree is identical to the one shown in your browser.  Also, notice that you can navigate to these files directly in the web browser -- the contents of the Git repo are served directly by Redwood (there are ways to handle staging/dev setups, but they're not covered by the demo).

Let's try pushing some commits.

- Try editing README.md, committing, and pushing.  You'll see that the browser updates the contents of the file instantly.
- Try copying a new JPEG image over `redwood.jpg`.  Commit and push.  The image will instantly update in the browser.

If you push updates to `index.html` or `script.js`, you'll need to refresh the demo page in the browser to see your changes (but they will still have occurred instantly).

