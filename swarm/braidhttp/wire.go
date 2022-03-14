package braidhttp

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type resourceRequest struct {
	Method          string              `method:""`
	StateURI        string              `header:"State-URI" query:"state_uri"`
	Version         *state.Version      `header:"Version"`
	AcceptHeader    AcceptHeader        `header:"Accept"`
	RangeHeader     *RangeRequest       `header:"Range"`
	KeypathAndRange keypathAndRangePath `path:""`
	Raw             bool                `query:"raw"`
}

type RangeType struct{ slug string }

var (
	RangeTypeBytes = RangeType{"bytes"}
	RangeTypeJSON  = RangeType{"json"}
)

type keypathAndRangePath struct {
	Keypath state.Keypath
	*RangeRequest
}

func (k *keypathAndRangePath) UnmarshalURLPath(path string) error {
	keypath, rng, err := state.ParseKeypathAndRange([]byte(path), byte('/'))
	if err != nil {
		return err
	}
	*k = keypathAndRangePath{Keypath: keypath}
	if rng != nil {
		k.RangeRequest = &RangeRequest{
			RangeType: RangeTypeJSON,
			Range:     &types.Range{rng.Start, rng.End, rng.Reverse},
			OpenEnded: false, // @@TODO
		}
	}
	return nil
}

type RangeRequest struct {
	RangeType RangeType    `json:"rangeType"`
	Range     *types.Range `json:"range"`
	OpenEnded bool         `json:"openEnded"`
}

func (r *RangeRequest) BytesRange() *types.Range {
	if r == nil {
		return nil
	}
	if r.RangeType == RangeTypeBytes {
		return r.Range
	}
	return nil
}

func (r *RangeRequest) JSONRange() *types.Range {
	if r == nil {
		return nil
	}
	if r.RangeType == RangeTypeJSON {
		return r.Range
	}
	return nil
}

func (r *RangeRequest) MarshalHTTPHeader() (string, bool, error) {
	if r.Range == nil {
		return "", false, nil
	}
	var header string
	if r.OpenEnded {
		header = fmt.Sprintf("%v-", r.Range.Start)
	} else {
		header = fmt.Sprintf("%v-%v", r.Range.Start, r.Range.End)
	}
	return fmt.Sprintf("%v=%v", r.RangeType, header), true, nil
}

func (r *RangeRequest) UnmarshalHTTPHeader(header string) error {
	*r = RangeRequest{}

	// Range: -10:-5
	// @@TODO: real json Range parsing
	parts := strings.SplitN(header, "=", 2)
	if len(parts) != 2 {
		return errors.Errorf("bad Range header: '%v'", header)
	}

	switch parts[0] {
	case "json":
		parts = strings.SplitN(parts[1], ":", 2)
		if len(parts) != 2 {
			return errors.Errorf("bad Range header: '%v'", header)
		}
		r.RangeType = RangeTypeJSON

	case "bytes":
		parts = strings.SplitN(parts[1], "-", 2)
		if len(parts) != 2 {
			return errors.Errorf("bad Range header: '%v'", header)
		}
		r.RangeType = RangeTypeBytes

	default:
		return errors.Errorf("bad Range header: '%v'", header)
	}
	start, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return errors.Errorf("bad Range header: '%v'", header)
	}
	if parts[1] == "" {
		if start == 0 {
			// e.g. "bytes=0-"
			r.Range = nil
		} else {
			r.Range = &types.Range{start, 0, false}
			r.OpenEnded = true
		}
	} else {
		end, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return errors.Errorf("bad Range header: '%v'", header)
		}
		r.Range = &types.Range{start, end, false}
	}
	return nil
}

type AcceptHeader types.Set[string]

func (h *AcceptHeader) UnmarshalHTTPHeader(header string) error {
	var trimmedAndFiltered []string
	for _, val := range strings.Split(header, ",") {
		trimmed := strings.TrimSpace(val)
		if len(trimmed) == 0 {
			continue
		}
		trimmedAndFiltered = append(trimmedAndFiltered, trimmed)
	}
	*h = AcceptHeader(types.NewSet[string](trimmedAndFiltered))
	return nil
}

func (h AcceptHeader) Contains(s string) bool {
	return types.Set[string](h).Contains(s)
}

type resourceResponse struct {
	Body           io.ReadCloser
	ContentType    string
	ContentLength  int64
	ResourceLength uint64
	StatusCode     int
}

type StoreBlobResponse struct {
	SHA1 types.Hash `json:"sha1"`
	SHA3 types.Hash `json:"sha3"`
}

type ParentsHeader types.Set[state.Version]

func (h *ParentsHeader) UnmarshalHTTPHeader(header string) error {
	ids := types.NewSet[state.Version](nil)
	for _, idStr := range strings.Split(header, ",") {
		id, err := state.VersionFromHex(strings.TrimSpace(idStr))
		if err != nil {
			return err
		}
		ids.Add(id)
	}
	*h = ParentsHeader(ids)
	return nil
}

func (h ParentsHeader) Slice() []state.Version {
	return types.Set[state.Version](h).Slice()
}
