(function (global) {

  var $global = $(global);
  var content, darkBox, searchInfo;
  var highlightOpts = { element: 'span', className: 'search-highlight' };

  var index = new lunr.Index;

  index.ref('id');
  index.field('title', { boost: 10 });
  index.field('body');
  index.pipeline.add(lunr.trimmer, lunr.stopWordFilter);

  $(populate);
  $(bind);

  function populate () {
    $('h1').each(function () {
      var title = $(this);
      var body = title.nextUntil('h1');
      var wrapper = $('<section id="section-' + title.prop('id') + '"></section>');

      title.after(wrapper.append(body));
      wrapper.prepend(title);

      index.add({
        id: title.prop('id'),
        title: title.text(),
        body: body.text()
      });
    });
  }

  function bind () {
    content = $('.content');
    darkBox = $('.dark-box');
    searchInfo = $('.search-info');

    $('#input-search')
      .on('keyup', search)
      .on('focus', active)
      .on('blur', inactive);
  }

  function refToHeader (itemRef) {
    return $('.tocify-item[data-unique=' + itemRef + ']').closest('.tocify-header');
  }

  function sortDescending (obj2, obj1) {
    var s1 = parseInt(obj1.id.replace(/[^\d]/g, ''), 10);
    var s2 = parseInt(obj2.id.replace(/[^\d]/g, ''), 10);
    return s1 === s2 ? 0 : s1 < s2 ? -1 : 1;
  }

  function resetHeaderLocations () {
    var headers = $(".tocify-header").sort(sortDescending);
    $.each(headers, function (index, item) {
      $(item).insertBefore($("#toc ul:first-child"));
    });
  }

  function search (event) {
    var sections = $('section, #toc .tocify-header');

    searchInfo.hide();
    unhighlight();

    // ESC clears the field
    if (event.keyCode === 27) this.value = '';

    if (this.value) {
      sections.hide();
      // results are sorted by score in descending order
      var tmpResults = index.search(this.value);
      var results = [];

      // remove low score matches
      $.each(tmpResults, function (index, item) {
        if (item.score >= 0.0001) results.push(item);
      });

      if (results.length) {
        lastRef = null;
        resetHeaderLocations();
        $.each(results, function (index, item) {
          var itemRef = item.ref;
          $('#section-' + itemRef).show();
          // headers must be repositioned in the DOM
          var closestHeader = refToHeader(itemRef);
          if (lastRef) {
            refToHeader(lastRef).insertBefore(closestHeader);
          }
          closestHeader.show();
          lastRef = itemRef;
        });

        // position first element. it wasn't positioned above if len > 1
        if (results.length > 1) {
          var firstRef = results[0].ref;
          var secondRef = results[1].ref
          refToHeader(firstRef).insertBefore(refToHeader(secondRef));
        }

        highlight.call(this);
      } else {
        sections.show();
        searchInfo.text('No Results Found for "' + this.value + '"').show();
      }
    } else {
      sections.show();
    }

    // HACK trigger tocify height recalculation
    $global.triggerHandler('scroll.tocify');
    $global.triggerHandler('resize');
  }

  function active () {
    search.call(this, {});
  }

  function inactive () {
    unhighlight();
    searchInfo.hide();
  }

  function highlight () {
    if (this.value) content.highlight(this.value, highlightOpts);
  }

  function unhighlight () {
    content.unhighlight(highlightOpts);
  }

})(window);
