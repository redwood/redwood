"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.useTreeReducer = void 0;
const react_1 = require("react");
const toolkit_1 = require("@reduxjs/toolkit");
const state_tree_initial_state_1 = require("./state-tree.initial-state");
const updateStateTree = (state, action) => {
    return Object.assign(Object.assign({}, state), { stateTrees: Object.assign(Object.assign({}, state.stateTrees), action.payload.newStateTree) });
};
const updateLeaves = (state, action) => {
    return Object.assign(Object.assign({}, state), { leaves: Object.assign(Object.assign({}, state.leaves), action.payload.newLeaves) });
};
const updateTreeAndLeaves = (state, action) => {
    const { newStateTree, newLeaves, stateURI } = action.payload;
    state.stateTrees[stateURI] = newStateTree;
    state.leaves[stateURI] = newLeaves;
    // const compareStateTree = { ...state.stateTrees, [stateURI]: newStateTree };
    // const compareLeaves = { ...state.stateTrees, [stateURI]: newLeaves };
    // const isStateTreeEqual =
    //     JSON.stringify(state.stateTrees) === JSON.stringify(compareStateTree);
    // const isLeavesEqual =
    //     JSON.stringify(state.leaves) === JSON.stringify(compareLeaves);
    // if (!isStateTreeEqual && !isLeavesEqual) {
    //     console.log("UPDATING statetree and leaves");
    //     state.stateTrees[stateURI] = newStateTree;
    //     state.leaves[stateURI] = newLeaves;
    // } else if (!isStateTreeEqual && isLeavesEqual) {
    //     console.log("UPDATING statetree");
    //     state.stateTrees[stateURI] = newStateTree;
    // } else if (isStateTreeEqual && !isLeavesEqual) {
    //     console.log("UPDATING leaves");
    //     state.leaves[stateURI] = newLeaves;
    // } else {
    //     console.log("not updating either");
    // }
    return state;
};
const updatePrivateTreeMembers = (state, action) => {
    const { stateURI, members } = action.payload;
    const newPrivateTreeMembers = Object.assign(Object.assign({}, state.privateTreeMembers), { [stateURI]: members });
    if (JSON.stringify(newPrivateTreeMembers) !==
        JSON.stringify(state.privateTreeMembers)) {
        state.privateTreeMembers[stateURI] = members;
    }
    return state;
};
const updateSubscribedStateURIs = (state, action) => {
    const { stateURI, isSubscribed } = action.payload;
    state.subscribedStateURIs[stateURI] = isSubscribed;
    return state;
};
const clearSubscribedStateURIs = (state, action) => {
    state.subscribedStateURIs = {};
    return state;
};
const resetTreeState = () => {
    return state_tree_initial_state_1.initialTreeState;
};
const getStateTree = (state, action) => {
    const { cb, key } = action.payload;
    if (key === "privateTreeMembers") {
        const safeKey = key;
        const safeState = toolkit_1.current(state[safeKey]);
        cb(safeState);
    }
    else {
        cb(toolkit_1.current(state));
    }
    return state;
};
const updateUnsubscribeList = (state, action) => {
    const { unsub, stateURI } = action.payload;
    state.unsubscribeList[stateURI] = unsub;
    return state;
};
const clearUnsubscribeList = (state, action) => {
    state.unsubscribeList = {};
};
const useTreeReducer = () => {
    const { name, reducer, actions, caseReducers } = toolkit_1.createSlice({
        name: "tree",
        initialState: state_tree_initial_state_1.initialTreeState,
        reducers: {
            updateStateTree,
            updateLeaves,
            updateTreeAndLeaves,
            updatePrivateTreeMembers,
            resetTreeState,
            getStateTree,
            updateSubscribedStateURIs,
            updateUnsubscribeList,
            clearSubscribedStateURIs,
            clearUnsubscribeList,
        },
    });
    const [state, dispatch] = react_1.useReducer(reducer, state_tree_initial_state_1.initialTreeState);
    return {
        name,
        actions,
        reducer,
        state,
        dispatch,
        caseReducers,
    };
};
exports.useTreeReducer = useTreeReducer;
exports.default = exports.useTreeReducer;
//# sourceMappingURL=index.js.map