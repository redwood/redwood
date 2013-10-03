Slate
========

Slate is a template for [Middleman](https://github.com/middleman/middleman) that turns a simple markdown file into [a beautiful API documentation page](TODOINSERTLINK).

For a real-world example, please see TripIt's API Documentation page, or the markdown file that is used to generate it. TODO ADD LINKS

Prerequisites
-------------

You're going to need:

 - **Ruby, version 2.0.0**
 - **Bundler**. If Ruby is already installed, but the bundle command doesn't work, just run `gem install bundler` in a terminal.

Getting Set Up
--------------

 1. Fork this repository on Github.
 1. Clone your repository with `git clone YOUR/GITHUB/URL/HERE`
 2. `cd api-docs`
 3. Install all dependencies: `bundle install`
 4. Start the test server: `bundle exec middleman server`

You can now see the docs at [http://localhost:4567]. Whoa! That was fast!

Making Edits
------------

Just edit the markdown file `source/index.md`! Super easy. Here's an example:

  # This is a Major Section Header

  This is a bunch of text that will appear in the light-background section to the left of your docs.

  ## This is a Header of One of Your API Endpoints

  Blah blah blah blah blah blah blah

  ## Another Endpoint Header

  More text here. This all appears in the area on the left.

  ### This is a Subheader (won't appear in the table of contents)

  ```ruby
  # this is a ruby code block that will appear in the dark area on the right
  def say_hello
    puts "hello"
  end
  ```

  ```python
  // this is a python code block that will appear in the dark area on the right
  print "hello, world"
  ```

  ```shell
  # this is a shell code block that will appear in the dark area on the right
  echo "hello, world"
  ```

  ```
  This is a generic text block that will appear in the dark area on the right
  ```

  > This is a code annotation that will appear in the dark area on the right

  This is a bunch of text that will appear in the light-background section to the left of your docs. This is [an internal link to one of the headers above](#another-endpoint-header).

  This is [an external link to google](http://google.com).

  This | Is  | A   | Table
  ---- | --- | --- | -----
  which will appear | in the area | to the left | of the code
  you can add | more rows | like this | (the vertical pipes don't need to line up)

Pushing Your Changes
--------------------

Publishing your changes couldn't be more simple.

 1. Commit your changes to the markdown source: `git commit -a`
 2. Push source changes to Github: `git push` (Note that at this point, we've only updated the ''markdown source'' on Github. We still haven't published the generated HTML to the `gh-pages` branch.)
 3. Build the site, commit, and push the `gh-pages` branch: `rake publish`

Done! Your changes should now be live on [http://yourusername.github.io/slate] TODO CHANGE URL, and the main branch should be updated with your edited markdown.

Don't Want to Use Github Pages?
-------------------------------

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