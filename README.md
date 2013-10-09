Slate
========

Slate turns a markdown file into a beautiful API documentation page for your service. Think of it as a documentation template on steroids.

Slate puts all your API's descriptions and definitions on left half of the screen, and all your code examples on the right half. Inspired by [Stripe's](https://stripe.com/docs/api) and [Paypal's](https://developer.paypal.com/webapps/developer/docs/api/) API docs.

If you have API bindings in multiple programming languages, you can easily make tabs at the top of the page to switch the code examples between those languages.

As an example, you can check out the [TripIt API docs](http://tripit.github.io/docs), which we create with Slate. You can also view the source of the [markdown file used to generate it](http://github.com/tripit/docs/blob/master/source/index.md).

### Prerequisites

You're going to need:

 - **Ruby, version 1.9.3 or newer**. It may work with older versions, but no guarantees.
 - **Bundler**. If Ruby is already installed, but the `bundle` command doesn't work, just run `gem install bundler` in a terminal.

### Getting Set Up

 1. Fork this repository on Github.
 1. Clone *your forked repository* (not our original one) to your hard drive with `git clone https://github.com/YOURUSERNAME/slate.git`
 2. `cd slate`
 3. Install all dependencies: `bundle install`
 4. Start the test server: `bundle exec middleman server`

You can now see the docs at <http://localhost:4567>. Whoa! That was fast!

### Making Edits

Just edit the markdown file `source/index.md`! If your middleman server is running, all you need to do is save `index.md` and reload the page in your browser to see the changes. Super easy.

Syntax-wise, if you know markdown, we think that the Kittn example in `source/index.md` is mostly self explanatory, but be sure to check out [the syntax documentation](http://github.com/tripit/slate/blob/master/syntax.md) if you need a better explanation.

### Publishing Your Docs

Publishing your API documentation couldn't be more simple.

 1. Commit your changes to the markdown source: `git commit -a -m "Update index.md"`
 2. Push the *markdown source* changes to Github: `git push`
 3. Compile to HTML, and push the HTML to Github pages: `rake publish`

Done! Your changes should now be live on <http://yourusername.github.io/slate>, and the main branch should be updated with your edited markdown. Note that if this is your first time publishing Slate, it can sometimes take ten minutes or so before your content is available online.

### Don't Want to Use Github Pages?

Instead of using `rake publish`, use `rake build`. Middleman will build your website to the `build` directory of your project, and you can copy those static HTML files to the server of your choice.

FAQ
----------------------------------

### How do I add programming languages to the tabs in the upper right corner?

At the top of `index.md`, just add more languages to the list under `language-tabs`.

Note that if a language is not found in `language-tabs`, we'll *always* display it. For instance, if your language tabs look like this:

    language_tabs:
      - shell
      - ruby

And you have code in your markdown that looks like this:

    ```shell
    echo "hello world"
    ```

    ```ruby
    puts "hello world"
    ```

    ```json
    {
      "hello":"world"
    }
    ```

Then the JSON will *always* be visible, since JSON isn't one of the language tabs.

### How do I totally hide the language tabs?

If you delete `language-tabs` altogether, we'll just completely hide the language bar.

### How do I change the display name of languages in the tabs?

Let's say your language tabs are like this:

    language_tabs:
      - shell
      - ruby
      - python

But you want "shell" to actually look like "cURL" in the tabs. You can't just change the titles of all the language blocks, since the syntax highlighter won't know what language it is. Instead, just change your `language_tabs` to look like this:

    language_tabs:
      - shell: cURL
      - ruby
      - python

Done! Language renamed. You can keep all your codeblocks for shell, but it'll look in the language bar like "cURL".

### How do I change the logo in the upper left?

Just replace `source/images/logo.png` with your logo.

### How do I get my code samples to line up with the content I want them next to?

Try putting code blocks and annotations right after headers. See the default `source/index.md` for an example, or file an issue if you're having trouble.

### You guys recently uploaded Slate. How can I incorporate your new changes into my fork?

Add the remote, call it "upstream":

    git remote add upstream git@github.com:tripit/slate.git

Fetch all the branches of that remote into remote-tracking branches, such as upstream/master:

    git fetch upstream

Make sure that you're on your master branch:

    git checkout master

Merge your our updates into your master branch:

    git merge upstream/master

Push the updated code to Github:

    git push

Publish the new changes to Github pages:

    rake publish

(Thanks to [Mark Longair on StackOverflow](http://stackoverflow.com/questions/7244321/how-to-update-github-forked-repository) for the upsteam explanation.)

Need Help?
--------------------

Just submit a issue to the Slate Github if you need any help. And feel free to submit pull requests with bug fixes or changes!
