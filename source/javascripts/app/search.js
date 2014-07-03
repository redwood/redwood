(function (global) {

  var $global = $(global);
  var content, darkBox, searchResults;
  var highlightOpts = { element: 'span', className: 'search-highlight' };

  var index = new lunr.Index();

  index.ref('id');
  index.field('title', { boost: 10 });
  index.field('body');
  index.pipeline.add(lunr.trimmer, lunr.stopWordFilter);

  $(populate);
  $(bind);

  function populate() {
    $('h1').each(function() {
      var title = $(this);
      var body = title.nextUntil('h1');
      index.add({
        id: title.prop('id'),
        title: title.text(),
        body: body.text()
      });
    });
  }

  function bind() {
    content = $('.content');
    darkBox = $('.dark-box');
    searchResults = $('.search-results');

    $('#input-search').on('keyup', function(e) {
      if ($(this).val() === "") {
        inactive(e);
      } else {
        search(e);
      }
    });
  }

  function search(event) {
    unhighlight();
    searchResults.addClass('visible');

    // ESC clears the field
    if (event.keyCode === 27) this.value = '';

    if (this.value) {
      var results = index.search(this.value);
      if (results.length) {
        $.each(results, function (index, result) {
          var header = $('.tocify-item[data-unique=' + result.ref + ']').closest('.tocify-header');
          if (header.length > 0) header = header[0];
          if (header) $("#" + header.id + " li a").append("<span>" + result.score + "</span>");
        });
        highlight.call(this);
      } else {
        searchResults.text('No Results Found for "' + this.value + '"').show();
      }
    }
  }

  function inactive() {
    unhighlight();
    searchResults.removeClass('visible');
  }

  function highlight() {
    if (this.value) content.highlight(this.value, highlightOpts);
  }

  function unhighlight() {
    content.unhighlight(highlightOpts);
  }

})(window);
