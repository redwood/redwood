function activateLanguage(language) {
  $("#lang-selector a").removeClass('active');
  $("#lang-selector a[data-language-name='" + language + "']").addClass('active');
  $(".highlight").hide();
  $(".highlight." + language).show();
  $(".highlight.text").show(); // always show text
}

function setupLanguages(languages) {
  currentLanguage = languages[0];

  activateLanguage(languages[0]);

  $("#lang-selector a").bind("click", function() {
    activateLanguage($(this).data("language-name"));
    return false;
  });

}
