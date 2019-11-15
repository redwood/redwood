
console.log('hello i am an extension')

chrome.runtime.onMessage.addListener(function(request, sender, callback) {
    console.log('content-script new state ~>', state)
    var preElem = document.querySelector('pre')
    var state = request.data
    preElem.innerHTML = JSON.stringify(state)
})


