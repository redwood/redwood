(function (global) {

  window.topic = topic;

  var index = lunr(function () {
    this.field('title', { boost: 10 });
    this.field('tags', { boost: 100 });
    this.field('body');
    this.ref('id');
  });

  $(bindSearch);

  function bindSearch () {
    $('#search').on('keyup', function () {
      if (this.value) {
        var items = index.search(this.value);
        $('article, nav li').hide();
        items.forEach(function (item) {
          $('#' + item.ref + ', #' + item.ref + '-nav').show();
        });
      } else {
        $('article, nav li').show();
      }
    });

    $('form').on('submit', function (event) {
      event.preventDefault();
    });
  }

  function topic (title, tags, handle) {
    index.add({
      id: handle,
      title: title,
      tags: tags,
      body: $('#' + handle + '-body').text()
    });
  }

})(window);
