import { TreeState } from "./state-tree.type";

export const initialTreeState: TreeState = {
    stateTrees: {},
    leaves: {},
    privateTreeMembers: {},
    subscribedStateURIs: {},
    unsubscribeList: {},
};
