Slate
========

Slate turns a markdown file into a beautiful API documentation page for your service.

This is much easier to show rather than explain, so as an example please see [TripIt's API Documentation page](http://tripit.github.io/docs), which we generated with Slate. You can also view the source of the [markdown file used to generate it](http://github.com/tripit/docs/blob/master/source/index.md).

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

Just edit the markdown file `source/index.md`! Super easy. Here's an example of some markdown:

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
    
    This is [an internal link to one of the headers above](#another-endpoint-header).
    
    This is [an external link to google](http://google.com).
    
    This | Is  | A   | Table
    ---- | --- | --- | -----
    which will appear | in the area | to the left | of the code
    you can add | more rows | like this | (the vertical pipes don't need to line up)
    
You can view the rendered version of the above example at [http://tripit.github.io/slate]

### Pushing Your Changes

Publishing your changes couldn't be more simple.

 1. Commit your changes to the markdown source: `git commit -a`
 2. Push source changes to Github: `git push` (Note that at this point, we've only updated the ''markdown source'' on Github. We still haven't published the generated HTML to the `gh-pages` branch.)
 3. Build the site, commit, and push the `gh-pages` branch: `rake publish`

Done! Your changes should now be live on [http://yourusername.github.io/slate] TODO CHANGE URL, and the main branch should be updated with your edited markdown.

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
