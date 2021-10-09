/// <reference types="react" />
import { StateTreeReducer, LeavesReducer, TreeAndLeavesReducer, PrivateTreeMembersReducer, ResetTreeReducer, GetStateTreeReducer, SubscribedStateURIsReducer, UpdateUnsubscribeReducer, TreeState } from "./state-tree.type";
export declare const useTreeReducer: () => {
    name: "tree";
    actions: import("@reduxjs/toolkit").CaseReducerActions<{
        updateStateTree: StateTreeReducer;
        updateLeaves: LeavesReducer;
        updateTreeAndLeaves: TreeAndLeavesReducer;
        updatePrivateTreeMembers: PrivateTreeMembersReducer;
        resetTreeState: ResetTreeReducer;
        getStateTree: GetStateTreeReducer;
        updateSubscribedStateURIs: SubscribedStateURIsReducer;
        updateUnsubscribeList: UpdateUnsubscribeReducer;
        clearSubscribedStateURIs: any;
        clearUnsubscribeList: any;
    }>;
    reducer: import("redux").Reducer<TreeState, import("redux").AnyAction>;
    state: TreeState;
    dispatch: import("react").Dispatch<import("redux").AnyAction>;
    caseReducers: {
        updateStateTree: StateTreeReducer;
        updateLeaves: LeavesReducer;
        updateTreeAndLeaves: TreeAndLeavesReducer;
        updatePrivateTreeMembers: PrivateTreeMembersReducer;
        resetTreeState: ResetTreeReducer;
        getStateTree: GetStateTreeReducer;
        updateSubscribedStateURIs: SubscribedStateURIsReducer;
        updateUnsubscribeList: UpdateUnsubscribeReducer;
        clearSubscribedStateURIs: any;
        clearUnsubscribeList: any;
    };
};
export default useTreeReducer;
//# sourceMappingURL=index.d.ts.map