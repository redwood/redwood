declare function sync9_create(): {
    T: {};
    leaves: {};
    val: null;
};
export function resolve_state(s9: any, sender: any, txHash: any, parents: any, patches: any): any;
declare function sync9_parse_change(change: any): {
    keys: never[];
};
declare function sync9_read(x: any, is_anc: any): any;
declare function sync9_add_version(x: any, vid: any, parents: any, changes: any, is_anc: any): void;
export { sync9_create as create, sync9_parse_change as parse_change, sync9_read as read, sync9_add_version as add_version };
//# sourceMappingURL=resolver.sync9.browser.d.ts.map