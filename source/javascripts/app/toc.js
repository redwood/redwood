$(function() {
  var toc = $("#toc").tocify({
    selectors: "h1,h2",
    extendPage: false,
    theme: "none",
    smoothScroll: false,
    showEffectSpeed: 0,
    hideEffectSpeed: 180,
    ignoreSelector: ".toc-ignore",
    hashGenerator: 'pretty',
    highlightOffset: 60,
    scrollTo: -2,
    scrollHistory: true,
    hashGenerator: function(text, element) {
      return element[0].getAttribute('id');
    }
  }).data("toc-tocify");

  // Hack to make already open sections to start opened,
  // instead of displaying an ugly animation
  setTimeout(function() {
    toc.setOption("showEffectSpeed", 180);
  }, 50);
});
