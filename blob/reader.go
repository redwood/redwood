package blob

import (
	"io"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type Reader struct {
	db           *state.DBTree
	manifest     Manifest
	spans        []readerSpan
	spanIdx      uint64
	chunkBytes   []byte
	chunkByteIdx uint64
	eof          bool
}

type readerSpan struct {
	ChunkIdx int
	Rng      types.Range
}

func NewReader(db *state.DBTree, manifest Manifest, rng *types.Range) *Reader {
	var spans []readerSpan

	if rng != nil {
		normalizedRange := rng.NormalizedForLength(manifest.TotalSize)

		var totalBytes uint64
		for i, chunk := range manifest.Chunks {
			intersection, exists := chunk.Range.Intersection(normalizedRange)
			if !exists {
				if len(spans) > 0 {
					break
				}
				totalBytes += chunk.Range.Length()
				continue
			}
			intersection.Start -= chunk.Range.Start
			intersection.End -= chunk.Range.Start
			spans = append(spans, readerSpan{ChunkIdx: i, Rng: intersection})
			totalBytes += intersection.Length()
		}
	} else {
		for i, chunk := range manifest.Chunks {
			spans = append(spans, readerSpan{ChunkIdx: i, Rng: chunk.Range})
		}
	}
	return &Reader{db: db, manifest: manifest, spans: spans}
}

func (r *Reader) Read(buf []byte) (int, error) {
	if r.eof {
		return 0, io.EOF
	}

	var copiedIntoBuf int
	for copiedIntoBuf < len(buf) {
		if r.spanIdx >= uint64(len(r.spans)) {
			r.eof = true
			break
		}

		if r.chunkBytes == nil {
			err := r.readNextChunk()
			if err != nil {
				return 0, err
			}
		}

		n := copy(buf[copiedIntoBuf:], r.chunkBytes[r.chunkByteIdx:])
		r.chunkByteIdx += uint64(n)
		copiedIntoBuf += n
		if r.chunkByteIdx >= uint64(len(r.chunkBytes)) {
			r.spanIdx++
			r.chunkBytes = nil
			r.chunkByteIdx = 0
		}
	}
	return copiedIntoBuf, nil
}

func (r *Reader) readNextChunk() error {
	node := r.db.State(false)
	defer node.Close()

	span := r.spans[r.spanIdx]
	chunk := r.manifest.Chunks[span.ChunkIdx]

	bytes, is, err := node.BytesValue(state.Keypath(chunk.SHA3.Hex()).Pushs("chunk"))
	if err != nil {
		return err
	} else if !is {
		return errors.New("value is not bytes")
	}
	bytesRange := span.Rng.NormalizedForLength(uint64(len(bytes)))
	r.chunkBytes = bytes[bytesRange.Start:bytesRange.End]
	return nil
}

func (r *Reader) Close() error {
	return nil
}
