
chrome.runtime.onMessage.addListener(function(request, sender, callback) {
    var preElem = document.querySelector('pre')
    var state = request.data
    preElem.innerHTML = JSON.stringify(state)
})


