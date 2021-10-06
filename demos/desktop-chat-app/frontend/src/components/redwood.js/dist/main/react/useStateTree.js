"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const react_1 = require("react");
const useRedwood_1 = __importDefault(require("./useRedwood"));
function useStateTree(stateURI, keypath) {
    const { redwoodClient, httpHost, useWebsocket, subscribe, subscribedStateURIs = {}, stateTrees, leaves, privateTreeMembers, updatePrivateTreeMembers, updateStateTree, getStateTree, } = useRedwood_1.default();
    const keypath_ = (keypath || "").length === 0 ? "/" : keypath;
    react_1.useEffect(() => {
        (function () {
            return __awaiter(this, void 0, void 0, function* () {
                if (!redwoodClient || !stateURI || !updatePrivateTreeMembers) {
                    return;
                }
                // @@TODO: just read from the `.Members` keypath
                if (!!redwoodClient.rpc) {
                    const rpc = redwoodClient.rpc;
                    getStateTree("privateTreeMembers", (currPTMembers) => {
                        // If stateURI do not exist on privateTreeMembers fetch members and add to state
                        if (!currPTMembers.hasOwnProperty(stateURI)) {
                            rpc.privateTreeMembers(stateURI).then((members) => {
                                updatePrivateTreeMembers(stateURI, members);
                            });
                        }
                    });
                }
            });
        })();
    }, [redwoodClient, stateURI]);
    react_1.useEffect(() => {
        if (!stateURI) {
            return;
        }
        subscribe(stateURI, (err, data) => {
            console.log(err, data);
        });
    }, [subscribe, stateURI, leaves, stateTrees]);
    return !!stateURI ? stateTrees[stateURI] : null;
}
exports.default = useStateTree;
//# sourceMappingURL=useStateTree.js.map