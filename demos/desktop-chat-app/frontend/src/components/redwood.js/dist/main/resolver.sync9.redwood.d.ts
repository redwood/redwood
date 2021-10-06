declare function init(internalState: any): void;
declare function resolve_state(state: any, sender: any, txHash: any, parents: any, patches: any): string;
declare function trimstr(s: any, n: any): any;
declare function sync9_extract_versions(s9: any, is_anc: any): {
    vid: null;
    parents: {};
    changes: string[];
}[];
declare function sync9_space_dag_extract_version(S: any, s9: any, vid: any, is_anc: any): any[];
declare function sync9_prune2(x: any, has_everyone_whos_seen_a_seen_b: any, has_everyone_whos_seen_a_seen_b2: any): {};
declare function sync9_space_dag_prune2(S: any, has_everyone_whos_seen_a_seen_b: any, seen_nodes: any): boolean;
declare function sync9_create(): {
    T: {};
    leaves: {};
    val: null;
};
declare function sync9_add_version(x: any, vid: any, parents: any, changes: any, is_anc: any): void;
declare function sync9_read(x: any, is_anc: any): any;
declare function sync9_create_space_dag_node(vid: any, elems: any, end_cap: any): {
    vid: any;
    elems: any;
    deleted_by: {};
    end_cap: any;
    nexts: never[];
    next: null;
};
declare function sync9_space_dag_get(S: any, i: any, is_anc: any): null;
declare function sync9_space_dag_set(S: any, i: any, v: any, is_anc: any): void;
declare function sync9_space_dag_length(S: any, is_anc: any): number;
declare function sync9_space_dag_break_node(node: any, x: any, end_cap: any, new_next: any): {
    vid: any;
    elems: any;
    deleted_by: {};
    end_cap: any;
    nexts: never[];
    next: null;
};
declare function sync9_space_dag_add_version(S: any, vid: any, splices: any, is_anc: any): void;
declare function sync9_trav_space_dag(S: any, f: any, cb: any, view_deleted: any, tail_cb: any): void;
declare function sync9_get_ancestors(x: any, vids: any): {};
declare function sync9_parse_change(change: any): {
    keys: never[];
};
declare function sync9_diff_ODI(a: any, b: any): any[][];
declare function sync9_guid(): string;
declare function sync9_create_proxy(x: any, cb: any, path: any): any;
declare function sync9_prune(x: any, has_everyone_whos_seen_a_seen_b: any, has_everyone_whos_seen_a_seen_b_2: any): {};
declare function sync9_space_dag_prune(S: any, has_everyone_whos_seen_a_seen_b: any, seen_nodes: any): boolean;
declare function binarySearch(ar: any, compare_fn: any): number;
declare function deep_equals(a: any, b: any): boolean;
declare var s9state: any;
declare var hasRun: boolean;
declare var logs: any[];
//# sourceMappingURL=resolver.sync9.redwood.d.ts.map