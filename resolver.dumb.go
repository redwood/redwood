package main

import (
	"encoding/json"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type dumbResolver struct{}

func NewDumbResolver() Resolver {
	return &dumbResolver{}
}

func (r *dumbResolver) ResolveState(state interface{}, p Patch) (interface{}, error) {
	j, _ := json.MarshalIndent(state, "", "    ")
	log.Errorln("RESOLVE STATE in", string(j))

	setval := func(val interface{}) { state = val }
	getval := func() interface{} { return state }

	if len(p.Keys) > 0 {
		var m map[string]interface{}

		if getval() == interface{}(nil) || getval() == nil {
			m = map[string]interface{}{}
			setval(m)
		} else {
			var ok bool
			m, ok = getval().(map[string]interface{})
			if !ok {
				m = map[string]interface{}{}
				setval(m)
			}
		}

		for i, key := range p.Keys {
			setval = func(val interface{}) { m[key] = val }
			getval = func() interface{} { return m[key] }

			if i == len(p.Keys)-1 {
				break
			}

			if m[key] == nil {
				old_m := m
				m = map[string]interface{}{}
				old_m[key] = m

			} else {
				new_m, ok := m[key].(map[string]interface{})
				if !ok {
					old_m := m
					m = map[string]interface{}{}
					old_m[key] = m

				} else {
					m = new_m
				}
			}
		}
	}

	if p.Range != nil {
		old_setval := setval
		setval = func(val interface{}) {

			switch v := val.(type) {
			case string:
				if getval() == nil {
					old_setval(val)
				} else {
					s, ok := getval().(string)
					if !ok {
						old_setval(val)
					} else if int64(len(s)) < p.Range[1] {
						old_setval(s[:p.Range[0]] + v)
					} else {
						old_setval(s[:p.Range[0]] + v + s[p.Range[1]:])
					}
				}

			case []interface{}:
				if getval() == nil {
					old_setval(val)
				} else {
					s, ok := getval().([]interface{})
					if !ok {
						old_setval(val)
					} else if int64(len(s)) < p.Range[1] {
						old_setval(append(s[:p.Range[0]], v...))
					} else {
						x := append(s[:p.Range[0]], v)
						old_setval(append(x, s[p.Range[1]:]...))
					}
				}

			default:
				panic(errors.Errorf("bad patch (type = %T)", p.Val))
			}
		}

	}

	setval(p.Val)

	j, _ = json.MarshalIndent(state, "", "    ")
	log.Errorln("RESOLVE STATE out", string(j))

	return state, nil
}
