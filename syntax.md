# Slate Syntax

## Headers

For headers:

    # Level 1 Header
    ## Level 2 Header
    ### Level 3 Header

Note that only level 1 and 2 headers will appear in the table of contents.

## Paragraph Text

For normal text, just type your paragraph on a single line.

    This is some paragraph text. Exciting, no?

Make sure the lines above below your paragraph are empty.

## Code Samples

For code samples:

    ```ruby
    # This is some Ruby code!
    ```

    ```python
    // This is some Python code!
    ```

Code samples will appear in the dark area to the left of the main text. We recommend positioning code samples right under headers in your markdown file.

For the full list of supported languages, see [rouge](http://rouge.jayferd.us/demo).

## Code Annotations

For code annotations:

    ```ruby
    # This is some Ruby code!
    ```

    > This is a code annotation. It will appear in the area to the right, next to the code samples.

    ```ruby
    # This is some more Ruby code!
    ```

Code annotations are essentially the same thing as paragraphs, but they'll appear in the area to the right along with your code samples.

## Tables

Slate uses PHP Markdown Extra style tables:

    Table Header 1 | Table Header 2 | Table Header 3
    -------------- | -------------- | --------------
    Row 1 col 1 | Row 1 col 2 | Row 1 col 3
    Row 2 col 1 | Row 2 col 2 | Row 2 col 3

Note that the pipes do not need to line up with each other on each line.

If you don't like that syntax, feel free to use normal html `<table>`s directly in your markdown.

## Formatting Text:

    This text is ***bold***, this is **italic**, this is an `inline code block`.

You can use those formatting rules in code annotations, tables, paragraphs, lists, wherever.

## Lists

    1. This
    2. Is
    3. An
    4. Ordered
    5. List

    * This
    * Is
    * A
    * Bullet
    * List

## Links

    This is an [internal link](#error-code-definitions), this is an [external link](http://google.com).
