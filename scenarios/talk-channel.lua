
--
-- patch:
--
--     messages[4:4] = {text: "asdf", timestamp: 12345}
--
--


function resolve_state(state, patch)
    if state == nil then
        state = {}
    end

    if patch:RangeStart() ~= -1 and patch:RangeStart() == patch:RangeEnd() then
        if state['messages'] == nil then
            state['messages'] = { patch.Val }

        elseif patch:RangeStart() <= #state['messages'] then
            -- state:Get('messages'):Insert(patch.Val)
            state['messages'][ #state['messages'] + 1 ] = patch.Val

        end
    else
        error('invalid')
    end
    return state
end
