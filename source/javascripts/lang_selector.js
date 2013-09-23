languages = []
function activateLanguage(language) {
  $("#lang-selector a").removeClass('active');
  $("#lang-selector a[data-language-name='" + language + "']").addClass('active');
  for (var i=0; i < languages.length; i++) {
    $(".highlight." + languages[i]).hide();
  }
  $(".highlight." + language).show();
}

function setupLanguages(l) {
  languages = l;
  currentLanguage = languages[0];

  if (location.search.substr(1) != "") {
    activateLanguage(location.search.substr(1));
  } else {
    activateLanguage(languages[0]);
  }

  $("#lang-selector a").bind("click", function() {
    window.location.replace("?" + $(this).data("language-name") + window.location.hash);
    return false;
  });

}

