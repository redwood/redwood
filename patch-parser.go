package redwood

import (
	"encoding/json"
	"strings"

	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"github.com/pkg/errors"
)

type parserPatch struct {
	Keys   parserPatchKeys   `@@`
	Range  *parserPatchRange `("[" @@ "]")?`
	Equals parserPatchEquals `@@`
}

type parserPatchParts struct {
}

type parserPatchKeys struct {
	Keys []string `("." @Ident | "[\" @Ident \"]")*`
}

type parserPatchRange struct {
	Start int64  `@Int`
	End   *int64 `(":" @Int)?`
}

type parserPatchEquals struct {
	Pos    lexer.Position
	Exists bool `@"="`
}

func ParsePatch(s string) (Patch, error) {
	idx := strings.Index(s, "=")
	if idx == -1 {
		return Patch{}, errors.New("wat")
	}

	keypath := s[:idx]
	jsonBlob := s[idx+1:]

	parser, err := participle.Build(&parserPatch{})
	if err != nil {
		return Patch{}, errors.WithStack(err)
	}

	var p parserPatch
	err = parser.ParseString(keypath, &p)
	if err != nil {
		// @@TODO: the fact that we don't try to consume the entire string makes the parser mad
		// return Patch{}, err
	}

	var rng *Range
	if p.Range != nil {
		end := p.Range.Start
		if p.Range.End != nil {
			end = *p.Range.End
		}
		rng = &Range{Start: p.Range.Start, End: end}
	}

	patch := Patch{
		Keys:  p.Keys.Keys,
		Range: rng,
	}

	jsonBlob = jsonEscape(jsonBlob)

	err = json.Unmarshal([]byte(jsonBlob), &patch.Val)
	if err != nil {
		panic(jsonBlob)
		return Patch{}, errors.WithStack(err)
	}

	return patch, nil
}

func jsonEscape(s string) string {
	retval := ""

	var inString bool
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && inString {
			retval += "\\n"
			continue
		}

		if s[i] == '"' {
			inString = !inString
		}
		retval += string(s[i])
	}
	return retval
}
