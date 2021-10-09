import { CaseReducer, PayloadAction } from "@reduxjs/toolkit";
import { UnsubscribeFunc } from "../..";
export declare type TreeStateObj = {
    [stateURI: string]: Object;
};
export declare type ArrayStringsType = {
    [stateURI: string]: string[];
};
export declare type PrivateTreeMembersObj = {
    [stateURI: string]: string[];
};
export declare type SubscribedStateURIsObj = {
    [stateURI: string]: boolean;
};
export declare type UnsubscribeListObj = {
    [stateURI: string]: Promise<UnsubscribeFunc>;
};
export declare type TreeState = {
    stateTrees: TreeStateObj;
    leaves: ArrayStringsType | any;
    privateTreeMembers: PrivateTreeMembersObj;
    subscribedStateURIs: SubscribedStateURIsObj;
    unsubscribeList: UnsubscribeListObj;
};
export declare type StateTreePayload = {
    newStateTree: Object;
};
export declare type LeavesPayload = {
    newLeaves: Object;
};
export declare type TreeAndLeavesPayload = {
    newStateTree: Object;
    newLeaves: Object;
    stateURI: string;
};
export declare type PrivateTreeMembersPayload = {
    stateURI: string;
    members: string[];
};
export declare type SubscribedStateURIsPayload = {
    stateURI: string;
    isSubscribed: boolean;
};
export declare type GetStateTreePayload = {
    cb: (state: Object) => void;
    key: keyof TreeState;
};
export declare type UnsubscribeListPayload = {
    stateURI: string;
    unsub: Promise<UnsubscribeFunc>;
};
export declare type StateTreeReducer = CaseReducer<TreeState, PayloadAction<StateTreePayload>>;
export declare type LeavesReducer = CaseReducer<TreeState, PayloadAction<LeavesPayload>>;
export declare type TreeAndLeavesReducer = CaseReducer<TreeState, PayloadAction<TreeAndLeavesPayload>>;
export declare type PrivateTreeMembersReducer = CaseReducer<TreeState, PayloadAction<PrivateTreeMembersPayload>>;
export declare type ResetTreeReducer = CaseReducer<TreeState, PayloadAction>;
export declare type SubscribedStateURIsReducer = CaseReducer<TreeState, PayloadAction<SubscribedStateURIsPayload>>;
export declare type GetStateTreeReducer = CaseReducer<TreeState, PayloadAction<GetStateTreePayload>>;
export declare type UpdateUnsubscribeReducer = CaseReducer<TreeState, PayloadAction<UnsubscribeListPayload>>;
//# sourceMappingURL=state-tree.type.d.ts.map