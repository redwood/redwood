Slate
========

Slate turns a markdown file into a beautiful API documentation page for your service. Think of it as a documentation template on steroids.

Slate puts all your API's descriptions and definitions on left half of the screen, and all your code examples on the right half. Inspired by [Stripe's](https://stripe.com/docs/api) and [Paypal's](https://developer.paypal.com/webapps/developer/docs/api/) API docs.

If you have API bindings in multiple programming languages, you can easily make tabs at the top of the page to switch the code examples between those languages.

As an example, you can check out our [API docs](http://tripit.github.io/docs), which we create with Slate. You can also view the source of the [markdown file used to generate it](http://github.com/tripit/docs/blob/master/source/index.md).

### Prerequisites

You're going to need:

 - **Ruby, version 2.0.0**
 - **Bundler**. If Ruby is already installed, but the bundle command doesn't work, just run `gem install bundler` in a terminal.

### Getting Set Up

 1. Fork this repository on Github.
 1. Clone your repository to your hard drive with `git clone https://github.com/YOURUSERNAME/slate.git`
 2. `cd slate`
 3. Install all dependencies: `bundle install`
 4. Start the test server: `bundle exec middleman server`

You can now see the docs at <http://localhost:4567>. Whoa! That was fast!

### Making Edits

Just edit the markdown file `source/index.md`! Super easy. If you know markdown, we think that the Kittn example in `source/index.md` is mostly self explanatory, but be sure to check out `syntax.md` if you need a better explanation.

### Publishing Your Docs

Publishing your API documentation couldn't be more simple.

 1. Commit your changes to the markdown source: `git commit -a`
 2. Push source changes to Github: `git push` (Note that at this point, we've only updated the ''markdown source'' on Github. We still haven't published the generated HTML to the `gh-pages` branch.)
 3. Build the site, commit, and push the `gh-pages` branch: `rake publish`

Done! Your changes should now be live on <http://yourusername.github.io/slate>, and the main branch should be updated with your edited markdown.

### Don't Want to Use Github Pages?

Instead of using `rake publish`, use `rake build`. Middleman will build your website to the `build` directory of your project, and you can copy those static HTML files to the server of your choice.

Extra Features
----------------------------------

### Adding Programming Languages

At the top of `index.md`, just add more languages to the list under `language-tabs`. Note that if a language is not found in `language-tabs`, we'll always display it regardless of which language is selected.

### Disable the Language Tabs

If you delete `language-tabs` altogether, we'll just completely hide the language bar.

### Custom Names for the Language Tabs

Let's say your language tabs are like this:

    language_tabs:
      - shell
      - ruby
      - python

But you want "shell" to actually look like "cURL" in the tabs. You can't just change the titles of all the language blocks, since the syntax highlighter won't know what language it is. Instead, just change your `language_tabs` to look like this:

    language_tabs:
      - shell:cURL
      - ruby
      - python

Done! Language renamed. You can keep all your codeblocks for shell, but it'll look in the language bar like "cURL".

### Change the Logo

Just replace `source/images/logo.png` with your logo.

### Getting the code to line up with the content you'd like it next to

Because of how Slate generates the document, we recommend you put the code you'd like to appear next to some content just before that content in the markdown. We normally put our code grouped together under second level headings.

## Need Help?

Just submit a issue to the Slate Github if you need any help. And feel free to submit pull requests with bug fixes or changes!
