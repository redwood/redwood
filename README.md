Slate
========

[![Build Status](https://travis-ci.org/tripit/slate.png?branch=master)](https://travis-ci.org/tripit/slate) [![Dependency Status](https://gemnasium.com/tripit/slate.png)](https://gemnasium.com/tripit/slate)

Slate helps you create beautiful single-page API documentation. Think of it as an intelligent, modern documentation template for your API.

<img src="https://dl.dropboxusercontent.com/u/95847291/github%20images/slate/slate_screenshot.png" width=700 alt="Screenshot of Example Documentation created with Slate">

*The example above was created with Slate. Check it out at [tripit.github.io/slate](http://tripit.github.io/slate).*

Features
------------

* **Clean, intuitive design** — with Slate, the description of your API is on the left side of your documentation, and all the code examples are on the right side. Inspired by [Stripe's](https://stripe.com/docs/api) and [Paypal's](https://developer.paypal.com/webapps/developer/docs/api/) API docs. In addition to the design you see on screen, Slate comes with a print stylesheet, so your docs look great on paper.

* **Everything on a single page** — gone are the days where your users had to search through a million pages to find what they wanted. Slate puts the entire documentation on a single page. We haven't sacrificed linkability, though. As you scroll, your browser's hash will update to the nearest header, so it's insanely easy to link to a particular point in the documentation.

* **Slate is just Markdown** — when you write docs with Slate, you're just writing Markdown, which makes it simple to edit and understand. Everything is written in Markdown — even the code samples are just Markdown code blocks!

* **Write code samples in multiple languages** — if your API has bindings in multiple programming languages, you easily put in tabs to switch between them. In your document, you'll distinguish different languages by specifying the language name at the top of each code block, just like with Github Flavored Markdown!

* **Out-of-the-box syntax highlighting** for [almost 60 languages](http://rouge.jayferd.us/demo), no configuration required.

* **Automatic, smoothly scrolling table of contents** on the far left of the page. As you scroll, it displays your current position in the document. It's fast, too. We're using Slate at TripIt to build documentation for our new API, where our table of contents has over 180 entries. We've made sure that the performance remains excellent, even for larger documents.

* **Let your users update your documentation for you** — by default, your Slate-generated documentation is hosted in a public Github repository. Not only does this mean you get free hosting for your docs with Github Pages, but it also makes it's simple for other developers to make pull requests to your docs if they find typos or other problems. Of course, if you don't want to, you're welcome to not use Github and host your docs elsewhere!

Getting starting with Slate is super easy! Simply fork this repository, and then follow the instructions below. Or, if you'd like to check out what Slate is capable of, take a look at the [sample docs](http://tripit.github.io/slate).

<!--As an example, you can check out the [TripIt API docs](http://tripit.github.io/docs), which we create with Slate. You can also view the source of the [markdown file used to generate it](http://github.com/tripit/docs/blob/master/source/index.md).-->

Getting Started with Slate
------------------------------

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

Syntax-wise, if you know markdown, we think that the Kittn example in `source/index.md` is mostly self explanatory, but be sure to check out [the syntax documentation](http://github.com/tripit/slate/blob/master/syntax.md) if you need a better explanation. Just remember — quotes and code samples appear on the right, everything else appears on the left.

### Publishing Your Docs to Github Pages

Publishing your API documentation couldn't be more simple.

 1. Commit your changes to the markdown source: `git commit -a -m "Update index.md"`
 2. Push the *markdown source* changes to Github: `git push`
 3. Compile to HTML, and push the HTML to Github pages: `rake publish`

Done! Your changes should now be live on http://yourusername.github.io/slate, and the main branch should be updated with your edited markdown. Note that if this is your first time publishing Slate, it can sometimes take ten minutes or so before your content is available online.

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

Just replace `source/images/logo.png` with your logo. We recommend a width of 230px and a height of around 50-100px. You can adjust the padding below the logo in `source/stylesheets/variables.scss`.

### How do I change the links that appear below the Table of Contents?

Just edit, at the top of `index.md`,

    toc_footers:
     - <a href='#'>Sign Up for a Developer Key</a>
     - <a href='http://github.com/tripit/slate'>Documentation Powered by Slate</a>

Add more lines or remove lines as needed. Just make sure they start with the dash. If you don't want any footer links at all, just remove `toc_footers` altogether, and the link section should just disappear.

Feel free to remove the "Documentation Powered by Slate" if you'd like, although we appreciate having that there since it helps more people discover Slate!

### How do I get my code samples to line up with the content I want them next to?

Try putting code blocks and annotations right after headers. See the default `source/index.md` for an example, or file an issue if you're having trouble.

### I want to change the colors!!!1

That's not exactly a *question*, but you can change the colors of various parts of the design, as well as some paddings, in `variables.scss`. This is also a great way to keep your changes to Slate's design in one place, so it will be easier to update Slate.

### You guys recently uploaded Slate. How can I incorporate your new changes into my fork?

Add the remote, call it `upstream`:

    git remote add upstream git@github.com:tripit/slate.git

Fetch all the branches of that remote into remote-tracking branches, such as `upstream/master`:

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

Need Help? Found a bug?
--------------------

Just [submit a issue](https://github.com/tripit/slate/issues) to the Slate Github if you need any help. And, of course, feel free to submit pull requests with bug fixes or changes.


Special Thanks
--------------------
- [Middleman](https://github.com/middleman/middleman)
- [jquery.tocify.js](https://github.com/gfranko/jquery.tocify.js)
- [middleman-syntax](https://github.com/middleman/middleman-syntax)
- [middleman-gh-pages](https://github.com/neo/middleman-gh-pages)
- [Font Awesome](http://fortawesome.github.io/Font-Awesome/)