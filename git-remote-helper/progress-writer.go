package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ijc25/Gotty"
)

type SingleLineWriter struct {
	out     io.Writer
	prevLen int
}

func NewSingleLineWriter(out io.Writer) *SingleLineWriter {
	return &SingleLineWriter{out: out}
}

func (w *SingleLineWriter) Printf(format string, args ...interface{}) {
	str := fmt.Sprintf(format, args...)
	newLen := len(str)

	if newLen < w.prevLen {
		spaces := strings.Repeat(" ", w.prevLen-newLen)
		str = str + spaces
	}

	fmt.Fprint(w.out, str+"\r")
	w.prevLen = newLen
}

type MultiLineWriter struct {
	out      io.Writer
	termInfo termInfo
	lines    int
	curLine  int
	first    bool
}

func NewMultiLineWriter(out io.Writer) *MultiLineWriter {
	term := os.Getenv("TERM")
	if term == "" {
		term = "vt102"
	}

	termInfo, err := gotty.OpenTermInfo(term)
	if err != nil {
		panic(err)
	}

	return &MultiLineWriter{
		out:      out,
		termInfo: termInfo,
		lines:    0,
	}
}

func (w *MultiLineWriter) Printf(line int, format string, args ...interface{}) {
	if line > w.lines {
		for i := 0; i < (line - w.lines); i++ {
			fmt.Fprintf(w.out, "\n")
		}
		w.lines = line
	}

	cursorUp(w.out, w.termInfo, w.lines-line)
	clearLine(w.out, w.termInfo)
	fmt.Fprintf(w.out, format+"\r", args...)
	cursorDown(w.out, w.termInfo, w.lines-line)
}

type termInfo interface {
	Parse(attr string, params ...interface{}) (string, error)
}

func clearLine(out io.Writer, ti termInfo) {
	// el2 (clear whole line) is not exposed by terminfo.

	// First clear line from beginning to cursor
	if attr, err := ti.Parse("el1"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[1K")
	}
	// Then clear line from cursor to end
	if attr, err := ti.Parse("el"); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[K")
	}
}

func cursorUp(out io.Writer, ti termInfo, l int) {
	if l == 0 { // Should never be the case, but be tolerant
		return
	}
	if attr, err := ti.Parse("cuu", l); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[%dA", l)
	}
}

func cursorDown(out io.Writer, ti termInfo, l int) {
	if l == 0 { // Should never be the case, but be tolerant
		return
	}
	if attr, err := ti.Parse("cud", l); err == nil {
		fmt.Fprintf(out, "%s", attr)
	} else {
		fmt.Fprintf(out, "\x1b[%dB", l)
	}
}
