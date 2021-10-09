import { parse_change } from './resolver.sync9.browser';
export { resolve_state, };
function init(internalState) { }
function resolve_state(state, sender, txID, parents, patches) {
    for (let pStr of patches) {
        let p = parse_change(pStr);
        let setval = function (val) { state = val; };
        let getval = function () { return state; };
        if (p.keys.length > 0) {
            let m = getval();
            if (m === null || m === undefined) {
                m = {};
                setval(m);
            }
            else {
                if (typeof m !== 'object') {
                    m = {};
                    setval(m);
                }
            }
            for (let i = 0; i < p.keys.length; i++) {
                let key = p.keys[i];
                setval = function (val) { m[key] = val; };
                getval = function () { return m[key]; };
                if (i === p.keys.length - 1) {
                    break;
                }
                if (m[key] === null || m[key] === undefined) {
                    m[key] = {};
                    m = m[key];
                }
                else {
                    if (typeof m[key] !== 'object') {
                        console.log('typeof m[key] !== object ~>', { m, key, state });
                        m[key] = {};
                        m = m[key];
                    }
                    else {
                        console.log('typeof m[key] === object ~>', { m, key, state });
                        m = m[key];
                    }
                }
            }
        }
        if (p.range) {
            let old_setval = setval;
            setval = function (val) {
                if (typeof val === 'string') {
                    if (getval() === null || getval() === undefined) {
                        old_setval(val);
                    }
                    else {
                        let s = getval();
                        if (typeof s !== 'string') {
                            old_setval(val);
                        }
                        else if (s.length < p.range[1]) {
                            old_setval(s.slice(0, p.range[0]) + val);
                        }
                        else {
                            old_setval(s.slice(0, p.range[0]) + val + s.slice(p.range[1]));
                        }
                    }
                }
                else {
                    if (getval() === null || getval() === undefined) {
                        old_setval(val);
                    }
                    else {
                        if (!(getval() instanceof Array)) {
                            old_setval(val);
                        }
                        if (val instanceof Array) {
                            if (getval().length < p.range[1]) {
                                old_setval([].concat(getval().slice(0, p.range[0]), val));
                            }
                            else {
                                let x = [].concat(getval().slice(0, p.range[0]), val);
                                old_setval([].concat(x, getval().slice(p.range[1])));
                            }
                        }
                        else {
                            if (getval().length < p.range[1]) {
                                old_setval([].concat(getval().slice(0, p.range[0]), [val]));
                            }
                            else {
                                let x = [].concat(getval().slice(0, p.range[0]), [val]);
                                old_setval([].concat(x, getval().slice(p.range[1])));
                            }
                        }
                    }
                }
            };
        }
        setval(p.val);
    }
    return state;
}
