require('@babel/polyfill')
const utils = require('./utils')


module.exports = {
    create: sync9_create,
    resolve_state: resolve_state,
    parse_change: sync9_parse_change,
    read: sync9_read,
    add_version: sync9_add_version,
}

// var p1 = `. = {"permissions":{"*":{"^.*$":{"read":true,"write":false}},"96216849c49358b10257cb55b28ea603c874b05e":{"^.*$":{"read":true,"write":true}}},"providers":["localhost:21231","localhost:21241"]}`
// var p2 = `.shrugisland = {}`
// var p3 = `.shrugisland.talk0 = {}`
// var p4 = sync9_parse_change(`.shrugisland.talk0.permissions = {"*":{"^\\\\.index.*$":{"read":true,"write":false},"^\\\\.messages.*":{"read":true,"write":true},"^\\\\.permissions.*$":{"read":true,"write":false}},"96216849c49358b10257cb55b28ea603c874b05e":{"^.*$":{"read":true,"write":true}}}`)
// var p5 = sync9_parse_change(`.shrugisland.talk0.messages = []`)
// var x = sync9_create()
// sync9_add_version(x, utils.genesisTxID, {}, [p1])
// sync9_add_version(x, 'p2', {[utils.genesisTxID]: true}, [p2, p3])
// sync9_add_version(x, 'p3', {p2: true}, [p3])
// sync9_add_version(x, 'p4', {p3: true}, [p4])
// sync9_add_version(x, 'p5', {p4: true}, [p5])
// console.log(sync9_read(x))

function resolve_state(s9, sender, txHash, parents, patches) {
    const parentsObj = {}
    if (parents && parents.length > 0) {
        parents.forEach(p => parentsObj[p] = true)
    }
    sync9_add_version(s9, txHash, parentsObj, patches, null)
    return sync9_read(s9)
}

function trimstr(s, n) {
    if (s.length > n) return s.substr(0, n)
    return s
}


function sync9_extract_versions(s9, is_anc) {
    var is_lit = x => !x || typeof(x) != 'object' || x.t == 'lit'
    var get_lit = x => (x && typeof(x) == 'object' && x.t == 'lit') ? x.S : x

    var versions = [{
        vid: null,
        parents: {},
        changes: [` = ${JSON.stringify(sync9_read(s9, is_anc))}`]
    }]
    Object.keys(s9.T).filter(x => !is_anc(x)).forEach(vid => {
        var ancs = sync9_get_ancestors(s9, {[vid]: true})
        delete ancs[vid]
        var is_anc = x => ancs[x]
        var path = []
        var changes = []
        rec(s9.val)
        function rec(x) {
            if (is_lit(x)) {
            } else if (x.t == 'val') {
                sync9_space_dag_extract_version(x.S, s9, vid, is_anc).forEach(s => {
                    if (s[2].length) changes.push(`${path.join('')} = ${JSON.stringify(s[2][0])}`)
                })
                sync9_trav_space_dag(x.S, is_anc, node => {
                    node.elems.forEach(rec)
                })
            } else if (x.t == 'arr') {
                sync9_space_dag_extract_version(x.S, s9, vid, is_anc).forEach(s => {
                    changes.push(`${path.join('')}[${s[0]}:${s[0] + s[1]}] = ${JSON.stringify(s[2])}`)
                })
                var i = 0
                sync9_trav_space_dag(x.S, is_anc, node => {
                    node.elems.forEach(e => {
                        path.push(`[${i++}]`)
                        rec(e)
                        path.pop()
                    })
                })
            } else if (x.t == 'obj') {
                Object.entries(x.S).forEach(e => {
                    path.push('[' + JSON.stringify(e[0]) + ']')
                    rec(e[1])
                    path.pop()
                })
            } else if (x.t == 'str') {
                sync9_space_dag_extract_version(x.S, s9, vid, is_anc).forEach(s => {
                    changes.push(`${path.join('')}[${s[0]}:${s[0] + s[1]}] = ${JSON.stringify(s[2])}`)
                })
            }
        }

        versions.push({
            vid,
            parents: Object.assign({}, s9.T[vid]),
            changes
        })
    })
    return versions
}

function sync9_space_dag_extract_version(S, s9, vid, is_anc) {
    var splices = []

    function add_result(offset, del, ins) {
        if (typeof(ins) != 'string')
            ins = ins.map(x => sync9_read(x, () => false))
        if (splices.length > 0) {
            var prev = splices[splices.length - 1]
            if (prev[0] + prev[1] == offset) {
                prev[1] += del
                prev[2] = prev[2].concat(ins)
                return
            }
        }
        splices.push([offset, del, ins])
    }

    var offset = 0
    function helper(node, _vid) {
        if (_vid == vid) {
            add_result(offset, 0, node.elems.slice(0))
        } else if (node.deleted_by[vid] && node.elems.length > 0) {
            add_result(offset, node.elems.length, node.elems.slice(0, 0))
        }

        if ((!_vid || is_anc(_vid)) && !Object.keys(node.deleted_by).some(is_anc)) {
            offset += node.elems.length
        }

        node.nexts.forEach(next => helper(next, next.vid))
        if (node.next) helper(node.next, _vid)
    }
    helper(S, null)
    return splices
}



function sync9_prune2(x, has_everyone_whos_seen_a_seen_b, has_everyone_whos_seen_a_seen_b2) {
    var seen_nodes = {}
    var is_lit = x => !x || typeof(x) != 'object' || x.t == 'lit'
    var get_lit = x => (x && typeof(x) == 'object' && x.t == 'lit') ? x.S : x
    function rec(x) {
        if (is_lit(x)) return x
        if (x.t == 'val') {
            sync9_space_dag_prune2(x.S, has_everyone_whos_seen_a_seen_b, seen_nodes)
            sync9_trav_space_dag(x.S, () => true, node => {
                node.elems = node.elems.slice(0, 1).map(rec)
            }, true)
            if (x.S.nexts.length == 0 && !x.S.next && x.S.elems.length == 1 && is_lit(x.S.elems[0])) return x.S.elems[0]
            return x
        }
        if (x.t == 'arr') {
            sync9_space_dag_prune2(x.S, has_everyone_whos_seen_a_seen_b, seen_nodes)
            sync9_trav_space_dag(x.S, () => true, node => {
                node.elems = node.elems.map(rec)
            }, true)
            if (x.S.nexts.length == 0 && !x.S.next && x.S.elems.every(is_lit) && !Object.keys(x.S.deleted_by).length) return {t: 'lit', S: x.S.elems.map(get_lit)}
            return x
        }
        if (x.t == 'obj') {
            Object.entries(x.S).forEach(e => x.S[e[0]] = rec(e[1]))
            if (Object.values(x.S).every(is_lit)) {
                var o = {}
                Object.entries(x.S).forEach(e => o[e[0]] = get_lit(e[1]))
                return {t: 'lit', S: o}
            }
            return x
        }
        if (x.t == 'str') {
            sync9_space_dag_prune2(x.S, has_everyone_whos_seen_a_seen_b, seen_nodes)
            if (x.S.nexts.length == 0 && !x.S.next && !Object.keys(x.S.deleted_by).length) return x.S.elems
            return x
        }
    }
    x.val = rec(x.val)

    var delete_us = {}
    var children = {}
    Object.keys(x.T).forEach(y => {
        Object.keys(x.T[y]).forEach(z => {
            if (!children[z]) children[z] = {}
            children[z][y] = true
        })
    })
    Object.keys(x.T).forEach(y => {
        if (!seen_nodes[y] && Object.keys(children[y] || {}).some(z => has_everyone_whos_seen_a_seen_b2(y, z))) delete_us[y] = true
    })

    var visited = {}
    var forwards = {}
    function g(vid) {
        if (visited[vid]) return
        visited[vid] = true
        if (delete_us[vid])
            forwards[vid] = {}
        Object.keys(x.T[vid]).forEach(pid => {
            g(pid)
            if (delete_us[vid]) {
                if (delete_us[pid])
                    Object.assign(forwards[vid], forwards[pid])
                else
                    forwards[vid][pid] = true
            } else if (delete_us[pid]) {
                delete x.T[vid][pid]
                Object.assign(x.T[vid], forwards[pid])
            }
        })
    }
    Object.keys(x.leaves).forEach(g)
    Object.keys(delete_us).forEach(vid => delete x.T[vid])
    return delete_us
}

function sync9_space_dag_prune2(S, has_everyone_whos_seen_a_seen_b, seen_nodes) {
    function set_nnnext(node, next) {
        while (node.next) node = node.next
        node.next = next
    }
    function process_node(node, offset, vid, prev) {
        var nexts = node.nexts
        var next = node.next

        var all_nexts_prunable = nexts.every(x => has_everyone_whos_seen_a_seen_b(vid, x.vid))
        if (nexts.length > 0 && all_nexts_prunable) {
            var first_prunable = 0
            var gamma = next
            if (first_prunable + 1 < nexts.length) {
                gamma = sync9_create_space_dag_node(null, typeof(node.elems) == 'string' ? '' : [])
                gamma.nexts = nexts.slice(first_prunable + 1)
                gamma.next = next
            }
            if (first_prunable == 0) {
                if (nexts[0].elems.length == 0 && !nexts[0].end_cap && nexts[0].nexts.length > 0) {
                    var beta = gamma
                    if (nexts[0].next) {
                        beta = nexts[0].next
                        set_nnnext(beta, gamma)
                    }
                    node.nexts = nexts[0].nexts
                    node.next = beta
                } else {
                    delete node.end_cap
                    node.nexts = []
                    node.next = nexts[0]
                    node.next.vid = null
                    set_nnnext(node, gamma)
                }
            } else {
                node.nexts = nexts.slice(0, first_prunable)
                node.next = nexts[first_prunable]
                node.next.vid = null
                set_nnnext(node, gamma)
            }
            return true
        }

        if (Object.keys(node.deleted_by).some(k => has_everyone_whos_seen_a_seen_b(vid, k))) {
            node.deleted_by = {}
            node.elems = node.elems.slice(0, 0)
            delete node.gash
            return true
        } else {
            Object.assign(seen_nodes, node.deleted_by)
        }

        if (next && !next.nexts[0] && (Object.keys(next.deleted_by).some(k => has_everyone_whos_seen_a_seen_b(vid, k)) || next.elems.length == 0)) {
            node.next = next.next
            return true
        }

        if (nexts.length == 0 && next &&
            !(next.elems.length == 0 && !next.end_cap && next.nexts.length > 0) &&
            Object.keys(node.deleted_by).every(x => next.deleted_by[x]) &&
            Object.keys(next.deleted_by).every(x => node.deleted_by[x])) {
            node.elems = node.elems.concat(next.elems)
            node.end_cap = next.end_cap
            node.nexts = next.nexts
            node.next = next.next
            return true
        }
    }

    var did_something_ever = false
    var did_something_this_time = true
    while (did_something_this_time) {
        did_something_this_time = false
        sync9_trav_space_dag(S, () => true, (node, offset, has_nexts, prev, vid) => {
            if (process_node(node, offset, vid, prev)) {
                did_something_this_time = true
                did_something_ever = true
            }
        }, true)
    }
    sync9_trav_space_dag(S, () => true, (node, offset, has_nexts, prev, vid) => {
        if (vid) seen_nodes[vid] = true
    }, true)
    return did_something_ever
}









function sync9_create() {
    return {
        T: {},
        leaves: {},
        val: null
    }
}

function sync9_add_version(x, vid, parents, changes, is_anc) {
    let make_lit = x => (x && typeof(x) == 'object') ? {t: 'lit', S: x} : x

    if (!vid && Object.keys(x.T).length == 0) {
        var parse = sync9_parse_change(changes[0])
        x.val = make_lit(changes[0].val)
        return
    } else if (!vid) return

    if (x.T[vid]) return
    x.T[vid] = Object.assign({}, parents)

    Object.keys(parents).forEach(k => {
        if (x.leaves[k]) delete x.leaves[k]
    })
    x.leaves[vid] = true

    if (!is_anc) {
        if (parents == x.leaves) {
            is_anc = (_vid) => _vid != vid
        } else {
            var ancs = sync9_get_ancestors(x, parents)
            is_anc = _vid => ancs[_vid]
        }
    }

    changes.forEach(change => {
        change = sync9_parse_change(change)
        var cur = x.val
        if (!cur || typeof(cur) != 'object' || cur.t == 'lit')
            cur = x.val = {t: 'val', S: sync9_create_space_dag_node(null, [cur])}
        var prev_S = null
        var prev_i = 0
        change.keys.forEach((key, i) => {
            if (cur.t == 'val') cur = sync9_space_dag_get(prev_S = cur.S, prev_i = 0, is_anc)
            if (cur.t == 'lit') {
                var new_cur = {}
                if (cur.S instanceof Array) {
                    new_cur.t = 'arr'
                    new_cur.S = sync9_create_space_dag_node(null, cur.S.map(x => make_lit(x)))
                } else {
                    if (typeof(cur.S) != 'object') throw 'bad'
                    new_cur.t = 'obj'
                    new_cur.S = {}
                    Object.entries(cur.S).forEach(e => new_cur.S[e[0]] = make_lit(e[1]))
                }
                cur = new_cur
                sync9_space_dag_set(prev_S, prev_i, cur, is_anc)
            }
            if (cur.t == 'obj') {
                let x = cur.S[key]
                if (!x || typeof(x) != 'object' || x.t == 'lit')
                    x = cur.S[key] = {t: 'val', S: sync9_create_space_dag_node(null, [x])}
                cur = x
            } else if (i == change.keys.length - 1 && !change.range) {
                change.range = [key, key + 1]
                change.val = (cur.t == 'str') ? change.val : [change.val]
            } else if (cur.t == 'arr') {
                cur = sync9_space_dag_get(prev_S = cur.S, prev_i = key, is_anc)
            } else throw 'bad'
        })
        if (!change.range) {
            if (cur.t != 'val') throw 'bad'
            var len = sync9_space_dag_length(cur.S, is_anc)
            sync9_space_dag_add_version(cur.S, vid, [[0, len, [make_lit(change.val)]]], is_anc)
        } else {
            if (cur.t == 'val') cur = sync9_space_dag_get(prev_S = cur.S, prev_i = 0, is_anc)
            if (typeof(cur) == 'string') {
                cur = {t: 'str', S: sync9_create_space_dag_node(null, cur)}
                sync9_space_dag_set(prev_S, prev_i, cur, is_anc)
            } else if (cur.t == 'lit') {
                if (!(cur.S instanceof Array)) throw 'bad'
                cur = {t: 'arr', S: sync9_create_space_dag_node(null, cur.S.map(x => make_lit(x)))}
                sync9_space_dag_set(prev_S, prev_i, cur, is_anc)
            }



            // work here
            if (change.val instanceof Array && cur.t != 'arr') {
                debugger
            }




            if (change.val instanceof String && cur.t != 'str') throw 'bad'
            if (change.val instanceof Array) change.val = change.val.map(x => make_lit(x))
            sync9_space_dag_add_version(cur.S, vid, [[change.range[0], change.range[1] - change.range[0], change.val]], is_anc)
        }
    })
}

function sync9_read(x, is_anc) {
    if (!is_anc) is_anc = () => true
    if (x && typeof(x) == 'object') {
        if (!x.t) return sync9_read(x.val, is_anc)
        if (x.t == 'lit') return x.S
        if (x.t == 'val') return sync9_read(sync9_space_dag_get(x.S, 0, is_anc), is_anc)
        if (x.t == 'obj') {
            var o = {}
            Object.entries(x.S).forEach(([k, v]) => {
                o[k] = sync9_read(v, is_anc)
            })
            return o
        }
        if (x.t == 'arr') {
            var a = []
            sync9_trav_space_dag(x.S, is_anc, (node) => {
                node.elems.forEach((e) => {
                    a.push(sync9_read(e, is_anc))
                })
            })
            return a
        }
        if (x.t == 'str') {
            var s = []
            sync9_trav_space_dag(x.S, is_anc, (node) => {
                s.push(node.elems)
            })
            return s.join('')
        }
        throw 'bad'
    } return x
}

function sync9_create_space_dag_node(vid, elems, end_cap) {
    return {
        vid : vid,
        elems : elems,
        deleted_by : {},
        end_cap : end_cap,
        nexts : [],
        next : null
    }
}

function sync9_space_dag_get(S, i, is_anc) {
    var ret = null
    var offset = 0
    sync9_trav_space_dag(S, is_anc ? is_anc : () => true, (node) => {
        if (i - offset < node.elems.length) {
            ret = node.elems[i - offset]
            return false
        }
        offset += node.elems.length
    })
    return ret
}

function sync9_space_dag_set(S, i, v, is_anc) {
    var offset = 0
    sync9_trav_space_dag(S, is_anc ? is_anc : () => true, (node) => {
        if (i - offset < node.elems.length) {
            node.elems[i - offset] = v
            return false
        }
        offset += node.elems.length
    })
}

function sync9_space_dag_length(S, is_anc) {
    var count = 0
    sync9_trav_space_dag(S, is_anc ? is_anc : () => true, node => {
        count += node.elems.length
    })
    return count
}

function sync9_space_dag_break_node(node, x, end_cap, new_next) {
    function subseq(x, start, stop) {
        return (x instanceof Array) ?
            x.slice(start, stop) :
            x.substring(start, stop)
    }

    var tail = sync9_create_space_dag_node(null, subseq(node.elems, x), node.end_cap)
    Object.assign(tail.deleted_by, node.deleted_by)
    tail.nexts = node.nexts
    tail.next = node.next

    node.elems = subseq(node.elems, 0, x)
    node.end_cap = end_cap
    if (end_cap) tail.gash = true
    node.nexts = new_next ? [new_next] : []
    node.next = tail

    return tail
}

function sync9_space_dag_add_version(S, vid, splices, is_anc) {

    function add_to_nexts(nexts, to) {
        var i = binarySearch(nexts, function (x) {
            if (to.vid < x.vid) return -1
            if (to.vid > x.vid) return 1
            return 0
        })
        nexts.splice(i, 0, to)
    }

    var si = 0
    var delete_up_to = 0

    var cb = (node, offset, has_nexts, prev, _vid, deleted) => {
        var s = splices[si]
        if (!s) return false

        if (deleted) {
            if (s[1] == 0 && s[0] == offset) {
                if (node.elems.length == 0 && !node.end_cap && has_nexts) return
                var new_node = sync9_create_space_dag_node(vid, s[2])
                if (node.elems.length == 0 && !node.end_cap)
                    add_to_nexts(node.nexts, new_node)
                else
                    sync9_space_dag_break_node(node, 0, undefined, new_node)
                si++
            }
            return
        }

        if (s[1] == 0) {
            var d = s[0] - (offset + node.elems.length)
            if (d > 0) return
            if (d == 0 && !node.end_cap && has_nexts) return
            var new_node = sync9_create_space_dag_node(vid, s[2])
            if (d == 0 && !node.end_cap) {
                add_to_nexts(node.nexts, new_node)
            } else {
                sync9_space_dag_break_node(node, s[0] - offset, undefined, new_node)
            }
            si++
            return
        }

        if (delete_up_to <= offset) {
            var d = s[0] - (offset + node.elems.length)
            if (d >= 0) return
            delete_up_to = s[0] + s[1]

            if (s[2]) {
                var new_node = sync9_create_space_dag_node(vid, s[2])
                if (s[0] == offset && node.gash) {
                    if (!prev.end_cap) throw 'no end_cap?'
                    add_to_nexts(prev.nexts, new_node)
                } else {
                    sync9_space_dag_break_node(node, s[0] - offset, true, new_node)
                    return
                }
            } else {
                if (s[0] == offset) {
                } else {
                    sync9_space_dag_break_node(node, s[0] - offset)
                    return
                }
            }
        }

        if (delete_up_to > offset) {
            if (delete_up_to <= offset + node.elems.length) {
                if (delete_up_to < offset + node.elems.length) {
                    sync9_space_dag_break_node(node, delete_up_to - offset)
                }
                si++
            }
            node.deleted_by[vid] = true
            return
        }
    }

    var f = is_anc
    var exit_early = {}
    var offset = 0
    function helper(node, prev, vid) {
        var has_nexts = node.nexts.find(next => f(next.vid))
        var deleted = Object.keys(node.deleted_by).some(vid => f(vid))
        if (cb(node, offset, has_nexts, prev, vid, deleted) == false)
            throw exit_early
        if (!deleted) {
            offset += node.elems.length
        }
        for (var next of node.nexts)
            if (f(next.vid)) helper(next, null, next.vid)
        if (node.next) helper(node.next, node, vid)
    }
    try {



        if (!S) {
            debugger
        }


        helper(S, null, S.vid)


    } catch (e) {
        if (e != exit_early) throw e
    }

}

function sync9_trav_space_dag(S, f, cb, view_deleted, tail_cb) {
    var exit_early = {}
    var offset = 0
    function helper(node, prev, vid) {
        var has_nexts = node.nexts.find(next => f(next.vid))
        if (view_deleted ||
            !Object.keys(node.deleted_by).some(vid => f(vid))) {
            if (cb(node, offset, has_nexts, prev, vid) == false)
                throw exit_early
            offset += node.elems.length
        }
        for (var next of node.nexts)
            if (f(next.vid)) helper(next, null, next.vid)
        if (node.next) helper(node.next, node, vid)
        else if (tail_cb) tail_cb(node)
    }
    try {
        helper(S, null, S.vid)
    } catch (e) {
        if (e != exit_early) throw e
    }
}

function sync9_get_ancestors(x, vids) {
    var ancs = {}
    function mark_ancs(key) {
        if (!ancs[key]) {
            ancs[key] = true
            Object.keys(x.T[key]).forEach(mark_ancs)
        }
    }
    Object.keys(vids).forEach(mark_ancs)
    return ancs
}

function sync9_parse_change(change) {
    var ret = { keys : [] }
    var re = /\.?([^\.\[ =]+)|\[((\-?\d+)(:\-?\d+)?|'(\\'|[^'])*'|"(\\"|[^"])*")\]|\s*=\s*(.*)/g
    var m
    while (m = re.exec(change)) {
        if (m[1])
            ret.keys.push(m[1])
        else if (m[2] && m[4])
            ret.range = [
                JSON.parse(m[3]),
                JSON.parse(m[4].substr(1))
            ]
        else if (m[2])
            ret.keys.push(JSON.parse(m[2]))
        else if (m[7])
            ret.val = JSON.parse(m[7])
    }

    return ret
}

function sync9_diff_ODI(a, b) {
    var offset = 0
    var prev = null
    var ret = []
    var d = diff_main(a, b)
    for (var i = 0; i < d.length; i++) {
        if (d[i][0] == 0) {
            if (prev) ret.push(prev)
            prev = null
            offset += d[i][1].length
        } else if (d[i][0] == 1) {
            if (prev)
                prev[2] += d[i][1]
            else
                prev = [offset, 0, d[i][1]]
        } else {
            if (prev)
                prev[1] += d[i][1].length
            else
                prev = [offset, d[i][1].length, '']
            offset += d[i][1].length
        }
    }
    if (prev) ret.push(prev)
    return ret
}

function sync9_guid() {
    var x = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'
    var s = []
    for (var i = 0; i < 15; i++)
        s.push(x[Math.floor(Math.random() * x.length)])
    return s.join('')
}

function sync9_create_proxy(x, cb, path) {
    path = path || ''
    var child_path = key => path + '[' + JSON.stringify(key) + ']'
    return new Proxy(x, {
        get : (x, key) => {
            if (['copyWithin', 'reverse', 'sort', 'fill'].includes(key))
                throw 'proxy does not support function: ' + key
            if (key == 'push') return function () {
                var args = Array.from(arguments)
                cb([path + '[' + x.length + ':' + x.length + '] = ' + JSON.stringify(args)])
                return x.push.apply(x, args)
            }
            if (key == 'pop') return function () {
                cb([path + '[' + (x.length - 1) + ':' + x.length + '] = []'])
                return x.pop()
            }
            if (key == 'shift') return function () {
                cb([path + '[0:1] = []'])
                return x.shift()
            }
            if (key == 'unshift') return function () {
                var args = Array.from(arguments)
                cb([path + '[0:0] = ' + JSON.stringify(args)])
                return x.unshift.apply(x, args)
            }
            if (key == 'splice') return function () {
                var args = Array.from(arguments)
                cb([child_path(key) + '[' + args[0] + ':' + (args[0] + args[1]) + '] = ' + JSON.stringify(args.slice(2))])
                return x.splice.apply(x, args)
            }

            var y = x[key]
            if (y && typeof(y) == 'object') {
                return sync9_create_proxy(y, cb, child_path(key))
            } else return y
        },
        set : (x, key, val) => {
            if (typeof(val) == 'string' && typeof(x[key]) == 'string') {
                cb(sync9_diff_ODI(x[key], val).map(splice => {
                    return child_path(key) + '[' + splice[0] + ':' + (splice[0] + splice[1]) + '] = ' + JSON.stringify(splice[2])
                }))
            } else {
                if ((x instanceof Array) && key.match(/^\d+$/)) key = +key
                cb([child_path(key) + ' = ' + JSON.stringify(val)])
            }
            x[key] = val
            return true
        }
    })
}

function sync9_prune(x, has_everyone_whos_seen_a_seen_b, has_everyone_whos_seen_a_seen_b_2) {
    var seen_nodes = {}
    var did_something = true
    function rec(x) {
        if (x && typeof(x) == 'object') {
            if (!x.t && x.val) {
                rec(x.val)
            } else if (x.t == 'val') {
                if (sync9_space_dag_prune(x.S, has_everyone_whos_seen_a_seen_b, seen_nodes)) did_something = true
                rec(sync9_space_dag_get(x.S, 0))
            } else if (x.t == 'obj') {
                Object.values(x.S).forEach(v => rec(v))
            } else if (x.t == 'arr') {
                if (sync9_space_dag_prune(x.S, has_everyone_whos_seen_a_seen_b, seen_nodes)) did_something = true
                sync9_trav_space_dag(x.S, () => true, node => {
                    node.elems.forEach(e => rec(e))
                })
            } else if (x.t == 'str') {
                if (sync9_space_dag_prune(x.S, has_everyone_whos_seen_a_seen_b, seen_nodes)) did_something = true
            }
        }
    }
    while (did_something) {
        did_something = false
        rec(x)
    }

    var visited = {}
    var delete_us = {}
    function f(vid) {
        if (visited[vid]) return
        visited[vid] = true
        Object.keys(x.T[vid]).forEach(pid => {
            if (has_everyone_whos_seen_a_seen_b_2(pid, vid) && !seen_nodes[pid]) {
                delete_us[pid] = true
            }
            f(pid)
        })
    }
    Object.keys(x.leaves).forEach(f)

    var visited = {}
    var forwards = {}
    function g(vid) {
        if (visited[vid]) return
        visited[vid] = true
        if (delete_us[vid])
            forwards[vid] = {}
        Object.keys(x.T[vid]).forEach(pid => {
            g(pid)
            if (delete_us[vid]) {
                if (delete_us[pid])
                    Object.assign(forwards[vid], forwards[pid])
                else
                    forwards[vid][pid] = true
            } else if (delete_us[pid]) {
                delete x.T[vid][pid]
                Object.assign(x.T[vid], forwards[pid])
            }
        })
    }
    Object.keys(x.leaves).forEach(g)
    Object.keys(delete_us).forEach(vid => delete x.T[vid])
    return delete_us
}

function sync9_space_dag_prune(S, has_everyone_whos_seen_a_seen_b, seen_nodes) {
    function set_nnnext(node, next) {
        while (node.next) node = node.next
        node.next = next
    }
    function process_node(node, offset, vid, prev) {
        var nexts = node.nexts
        var next = node.next

        var first_prunable = nexts.findIndex(x => has_everyone_whos_seen_a_seen_b(vid, x.vid))
        if (first_prunable > 0 && (node.elems.length > 0 || !prev)) {
            first_prunable = nexts.findIndex((x, i) => (i > first_prunable) && has_everyone_whos_seen_a_seen_b(vid, x.vid))
        }

        if (first_prunable >= 0) {
            var gamma = next
            if (first_prunable + 1 < nexts.length) {
                gamma = sync9_create_space_dag_node(null, typeof(node.elems) == 'string' ? '' : [])
                gamma.nexts = nexts.slice(first_prunable + 1)
                gamma.next = next
            }
            if (first_prunable == 0) {
                if (nexts[0].elems.length == 0 && !nexts[0].end_cap && nexts[0].nexts.length > 0) {
                    var beta = gamma
                    if (nexts[0].next) {
                        beta = nexts[0].next
                        set_nnnext(beta, gamma)
                    }
                    node.nexts = nexts[0].nexts
                    node.next = beta
                } else {
                    delete node.end_cap
                    node.nexts = []
                    node.next = nexts[0]
                    node.next.vid = null
                    set_nnnext(node, gamma)
                }
            } else {
                node.nexts = nexts.slice(0, first_prunable)
                node.next = nexts[first_prunable]
                node.next.vid = null
                set_nnnext(node, gamma)
            }
            return true
        }

        if (Object.keys(node.deleted_by).some(k => has_everyone_whos_seen_a_seen_b(vid, k))) {
            node.deleted_by = {}
            node.elems = typeof(node.elems) == 'string' ? '' : []
            delete node.gash
            return true
        } else {
            Object.assign(seen_nodes, node.deleted_by)
        }

        if (next && !next.nexts[0] && (Object.keys(next.deleted_by).some(k => has_everyone_whos_seen_a_seen_b(vid, k)) || next.elems.length == 0)) {
            node.next = next.next
            return true
        }

        if (nexts.length == 0 && next &&
            !(next.elems.length == 0 && !next.end_cap && next.nexts.length > 0) &&
            Object.keys(node.deleted_by).every(x => next.deleted_by[x]) &&
            Object.keys(next.deleted_by).every(x => node.deleted_by[x])) {
            node.elems = node.elems.concat(next.elems)
            node.end_cap = next.end_cap
            node.nexts = next.nexts
            node.next = next.next
            return true
        }
    }
    var did_something = false
    sync9_trav_space_dag(S, () => true, (node, offset, has_nexts, prev, vid) => {
        if (!prev) seen_nodes[vid] = true
        while (process_node(node, offset, vid, prev)) {
            did_something = true
        }
    }, true)
    return did_something
}

// modified from https://stackoverflow.com/questions/22697936/binary-search-in-javascript
function binarySearch(ar, compare_fn) {
    var m = 0;
    var n = ar.length - 1;
    while (m <= n) {
        var k = (n + m) >> 1;
        var cmp = compare_fn(ar[k]);
        if (cmp > 0) {
            m = k + 1;
        } else if(cmp < 0) {
            n = k - 1;
        } else {
            return k;
        }
    }
    return m;
}






function deep_equals(a, b) {
    if (typeof(a) != 'object' || typeof(b) != 'object') return a == b
    if (a == null) return b == null
    if (Array.isArray(a)) {
        if (!Array.isArray(b)) return false
        if (a.length != b.length) return false
        for (var i = 0; i < a.length; i++)
            if (!deep_equals(a[i], b[i])) return false
        return true
    }
    var ak = Object.keys(a).sort()
    var bk = Object.keys(b).sort()
    if (ak.length != bk.length) return false
    for (var k of ak)
        if (!deep_equals(a[k], b[k])) return false
    return true
}
