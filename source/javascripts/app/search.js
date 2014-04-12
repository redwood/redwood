(function (global) {

  var $global = $(global);
  var index = lunr(function () {
    this.ref('id');
    this.field('title', { boost: 10 });
    // this.field('tags', { boost: 100 });
    this.field('body');
  });

  $(populate);
  $(bind);

  function populate () {
    $('h1').each(function () {
      var title = $(this);
      var body = title.nextUntil('h1, .search-nothing-found');
      var wrapper = $('<section id="section-' + title.prop('id') + '"></section>');

      title.after(wrapper.append(body));
      wrapper.prepend(title);

      index.add({
        id: title.prop('id'),
        title: title.text(),
        // tags: tags,
        body: body.text()
      });
    });
  }

  function bind () {
    $('#input-search').on('keyup', search);
  }

  function search (event) {
    var $sections = $('section, #toc .tocify-header');
    var $content = $('.content');
    var opts = { element: 'span', className: 'search-highlight' };

    $content.unhighlight(opts);

    // esc clears the field
    if (event.keyCode === 27) this.value = '';

    if (this.value) {
      var items = index.search(this.value);
      $sections.hide();
      if (items.length) {
        items.forEach(function (item) {
          $('#section-' + item.ref).show();
          $('.tocify-item[data-unique=' + item.ref + ']').closest('.tocify-header').show();
        });
        $content.highlight(this.value, opts);
      } else {
        $sections.filter('.search-nothing-found').show();
      }
    } else {
      $sections.show();
    }

    // HACK trigger tocify height recalculation
    $global.triggerHandler('scroll.tocify');
    $global.triggerHandler('resize');
  }

})(window);
