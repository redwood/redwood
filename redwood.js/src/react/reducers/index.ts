import { useReducer } from "react";
import { createSlice, current } from "@reduxjs/toolkit";

import {
    StateTreeReducer,
    LeavesReducer,
    TreeAndLeavesReducer,
    PrivateTreeMembersReducer,
    ResetTreeReducer,
    GetStateTreeReducer,
    SubscribedStateURIsReducer,
    UpdateUnsubscribeReducer,
    TreeState,
} from "./state-tree.type";
import { initialTreeState } from "./state-tree.initial-state";

const updateStateTree: StateTreeReducer = (state, action) => {
    return {
        ...state,
        stateTrees: {
            ...state.stateTrees,
            ...action.payload.newStateTree,
        },
    };
};

const updateLeaves: LeavesReducer = (state, action) => {
    return {
        ...state,
        leaves: {
            ...state.leaves,
            ...action.payload.newLeaves,
        },
    };
};

const updateTreeAndLeaves: TreeAndLeavesReducer = (state, action) => {
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

const updatePrivateTreeMembers: PrivateTreeMembersReducer = (state, action) => {
    const { stateURI, members } = action.payload;

    const newPrivateTreeMembers = {
        ...state.privateTreeMembers,
        [stateURI]: members,
    };

    if (
        JSON.stringify(newPrivateTreeMembers) !==
        JSON.stringify(state.privateTreeMembers)
    ) {
        state.privateTreeMembers[stateURI] = members;
    }

    return state;
};

const updateSubscribedStateURIs: SubscribedStateURIsReducer = (
    state,
    action
) => {
    const { stateURI, isSubscribed } = action.payload;

    state.subscribedStateURIs[stateURI] = isSubscribed;

    return state;
};

const clearSubscribedStateURIs: any = (state: any, action: any) => {
    state.subscribedStateURIs = {};

    return state;
};

const resetTreeState: ResetTreeReducer = () => {
    return initialTreeState;
};

const getStateTree: GetStateTreeReducer = (state, action) => {
    const { cb, key } = action.payload;

    if (key === "privateTreeMembers") {
        const safeKey: keyof TreeState = key;
        const safeState = current(state[safeKey] || {});
        console.log("another");
        cb(safeState);
    } else {
        cb(current(state));
    }

    return state;
};

const updateUnsubscribeList: UpdateUnsubscribeReducer = (state, action) => {
    const { unsub, stateURI } = action.payload;

    state.unsubscribeList[stateURI] = unsub;

    return state;
};

const clearUnsubscribeList: any = (state: any, action: any) => {
    state.unsubscribeList = {};
};

export const useTreeReducer = () => {
    const { name, reducer, actions, caseReducers } = createSlice({
        name: "tree",
        initialState: initialTreeState,
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

    const [state, dispatch] = useReducer(reducer, initialTreeState);

    return {
        name,
        actions,
        reducer,
        state,
        dispatch,
        caseReducers,
    };
};

export default useTreeReducer;
