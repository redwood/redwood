package main

type (
	ID [32]byte

	Tx struct {
		ID ID

		ParentsLen int `struc:"sizeof=Parents"`
		Parents    []ID

		PatchStringsLen int `struc:"sizeof=PatchStrings"`
		PatchStrings    []string

		Patches []Patch `struc:"-"`
	}

	Patch struct {
		Keys  []string
		Range *[2]int64
		Val   interface{}
	}
)

func (tx *Tx) EncodeToWire() error {
	tx.PatchStrings = make([]string, len(tx.Patches))
	for i, patch := range tx.Patches {
		tx.PatchStrings[i] = patch.String()
	}
	return nil
}

func (tx *Tx) DecodeFromWire() error {
	tx.Patches = make([]string, len(tx.PatchStrings))
	for i, patchStr := range tx.PatchStrings {
		patch, err := ParsePatch(patchStr)
		if err != nil {
			return err
		}
		tx.Patches[i] = patch
	}
}

var (
	patchRegexp = regexp.MustCompile(`\.?([^\.\[ =]+)|\[((\-?\d+)(:\-?\d+)?|'(\\'|[^'])*'|"(\\"|[^"])*")\]|\s*=\s*(.*)`)
	ErrBadPatch = errors.New("bad patch string")
)

func ParsePatch(txt string) (Patch, error) {
	matches := patchRegexp.FindAllStringSubmatch(txt, -1)

	var patch Patch
	for _, m := range matches {
		switch {
		case len(m[1]) > 0:
			patch.Keys = append(patch.Keys, m[1])

		case len(m[2]) > 0 && len(m[4]) > 0:
			start, err := strconv.ParseInt(m[2], 10, 64)
			if err != nil {
				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
			}
			end, err := strconv.ParseInt(m[4][1:], 10, 64)
			if err != nil {
				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
			}

			patch.Range = &Range{start, end}

		case len(m[2]) > 0:
			patch.Keys = append(patch.Keys, m[2][1:len(m[2])-1])

		case len(m[7]) > 0:
			err := json.Unmarshal([]byte(m[7]), &patch.Val)
			if err != nil {
				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
			}

		default:
			return Patch{}, errors.Wrap(ErrBadPatch, "syntax error")
		}
	}

	return patch, nil
}

func (p Patch) String() string {
	var s string
	for i := range p.Keys {
		s += "." + p.Keys[i]
	}

	if p.Range != nil {
		s += fmt.Sprintf("[%d:%d]", p.Range[0], p.Range[1])
	}

	val, err := json.Marshal(p.Val)
	if err != nil {
		panic(err)
	}

	s += " = " + string(val)

	return s
}
