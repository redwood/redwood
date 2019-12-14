
function resolve_state(state, sender, patch)
    if state == nil then
        state = {}
    end

    if patch:RangeStart() ~= -1 and patch:RangeStart() == patch:RangeEnd() then
        local msg = {
            text = patch.Val['text'],
            sender = sender,
        }
        if state['messages'] == nil then
            state['messages'] = { msg }

        elseif patch:RangeStart() <= #state['messages'] then
            state['messages'][ #state['messages'] + 1 ] = msg

        end
    else
        error('only patches that append messages to the end of the channel are allowed')
    end
    return state
end
