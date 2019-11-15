
function init(internalStateJSON) {}

function resolve_state(stateJSON, sender, txHash, parents, patches) {
    var state = JSON.parse(stateJSON)

    patches.forEach(function(patch) {
        state.messages.push({
            text: patch.val[0].text,
            sender: sender,
        })
    })

    return JSON.stringify({ state: state, internalState: {} })
}
