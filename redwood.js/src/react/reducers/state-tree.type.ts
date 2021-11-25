import {
    CaseReducer,
    CaseReducerActions,
    PayloadAction,
} from "@reduxjs/toolkit";

import { UnsubscribeFunc } from "../..";

export type TreeStateObj = { [stateURI: string]: Object };
export type ArrayStringsType = { [stateURI: string]: string[] };
export type PrivateTreeMembersObj = { [stateURI: string]: string[] };
export type SubscribedStateURIsObj = { [stateURI: string]: boolean };
export type UnsubscribeListObj = {
    [stateURI: string]: Promise<UnsubscribeFunc>;
};

export type TreeState = {
    stateTrees: TreeStateObj;
    leaves: ArrayStringsType | any;
    privateTreeMembers: PrivateTreeMembersObj;
    subscribedStateURIs: SubscribedStateURIsObj;
    unsubscribeList: UnsubscribeListObj;
};

export type StateTreePayload = {
    newStateTree: Object;
};

export type LeavesPayload = {
    newLeaves: Object;
};

export type TreeAndLeavesPayload = {
    newStateTree: Object;
    newLeaves: Object;
    stateURI: string;
};

export type PrivateTreeMembersPayload = {
    stateURI: string;
    members: string[];
};

export type SubscribedStateURIsPayload = {
    stateURI: string;
    isSubscribed: boolean;
};

export type GetStateTreePayload = {
    cb: (state: Object) => void;
    key: keyof TreeState;
};

export type UnsubscribeListPayload = {
    stateURI: string;
    unsub: Promise<UnsubscribeFunc>;
};

export type StateTreeReducer = CaseReducer<
    TreeState,
    PayloadAction<StateTreePayload>
>;

export type LeavesReducer = CaseReducer<
    TreeState,
    PayloadAction<LeavesPayload>
>;

export type TreeAndLeavesReducer = CaseReducer<
    TreeState,
    PayloadAction<TreeAndLeavesPayload>
>;

export type PrivateTreeMembersReducer = CaseReducer<
    TreeState,
    PayloadAction<PrivateTreeMembersPayload>
>;

export type ResetTreeReducer = CaseReducer<TreeState, PayloadAction>;

export type SubscribedStateURIsReducer = CaseReducer<
    TreeState,
    PayloadAction<SubscribedStateURIsPayload>
>;

export type GetStateTreeReducer = CaseReducer<
    TreeState,
    PayloadAction<GetStateTreePayload>
>;

export type UpdateUnsubscribeReducer = CaseReducer<
    TreeState,
    PayloadAction<UnsubscribeListPayload>
>;
