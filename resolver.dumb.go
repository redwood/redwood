package redwood

type dumbResolver struct{}

func NewDumbResolver(params map[string]interface{}) (Resolver, error) {
	return &dumbResolver{}, nil
}

func (r *dumbResolver) ResolveState(state interface{}, sender Address, p Patch) (interface{}, error) {
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
					} else if int64(len(s)) < p.Range.End {
						old_setval(s[:p.Range.Start] + v)
					} else {
						old_setval(s[:p.Range.Start] + v + s[p.Range.End:])
					}
				}

			default:
				if getval() == nil {
					old_setval(val)
				} else {
					s, ok := getval().([]interface{})
					if !ok {
						old_setval(val)
					}

					seq, isSequence := v.([]interface{})
					if isSequence {
						if int64(len(s)) < p.Range.End {
							old_setval(append(s[:p.Range.Start], seq...))
						} else {
							x := append(s[:p.Range.Start], seq...)
							old_setval(append(x, s[p.Range.End:]...))
						}

					} else {
						if int64(len(s)) < p.Range.End {
							old_setval(append(s[:p.Range.Start], v))
						} else {
							x := append(s[:p.Range.Start], v)
							old_setval(append(x, s[p.Range.End:]...))
						}
					}
				}
			}
		}

	}

	setval(p.Val)

	return state, nil
}
