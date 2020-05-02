package main

import (
	"bytes"
	"time"
)

// FmtConstWidth is a basic formatter that makes reasonable attempts to make the header length a constant width,
// improving readability. It also can insert console color codes so each severity level is a different color.
type FmtConstWidth struct {
	// FileNameCharWidth is the number of chars to use from the given file name.
	// Filenames shorter than this are padded with spaces.
	// If 0, file names are not printed.
	FileNameCharWidth int

	// If set, color codes will be inserted
	UseColor bool
}

// FormatHeader -- see interface Formatter
func (f *FmtConstWidth) FormatHeader(inSeverity string, inFile string, inLine int, buf *bytes.Buffer) {
	var (
		tmp [64]byte
	)

	sevChar := inSeverity[0]
	sz := 0

	// Add a severity char that we'll remove before displaying.
	// It's used by the logFilter in termUI.
	tmp[sz] = sevChar
	sz++

	usingColor := f.UseColor
	if usingColor {
		tmp[sz] = '['
		sz++
	}

	tmp[sz] = sevChar
	sz++

	sz += AppendTimestamp(tmp[sz:])
	tmp[sz] = ' '
	sz++
	buf.Write(tmp[:sz])
	sz = 0

	if segSz := f.FileNameCharWidth; segSz > 0 {
		strLen := len(inFile)
		padLen := segSz - strLen
		if padLen < 0 {
			buf.Write([]byte(inFile))
		} else {
			for ; sz < padLen; sz++ {
				tmp[sz] = ' '
			}
			for ; sz < segSz; sz++ {
				tmp[sz] = inFile[sz-padLen]
			}
		}
		tmp[sz] = ':'
		sz++
		if inLine < 10000 {
			sz += AppendNDigits(4, inLine, tmp[sz:], '0')
		} else {
			sz += AppendDigits(inLine, tmp[sz:])
		}
	}
	// tmp[sz] = ']'
	// tmp[sz+1] = ' '
	// sz += 2

	if usingColor {
		// sz += AppendColorCode(byte(noColor), tmp[sz:])
		tmp[sz] = ']'
		sz++
		switch sevChar {
		case 'W':
			copy(tmp[sz:], []byte("(fg:yellow)"))
			sz += 11
		case 'E', 'F':
			copy(tmp[sz:], []byte("(fg:red)"))
			sz += 8
		case 'S':
			copy(tmp[sz:], []byte("(fg:green)"))
			sz += 10
		case 'D':
			copy(tmp[sz:], []byte("(fg:magenta)"))
			sz += 12
		default:
			copy(tmp[sz:], []byte("(fg:clear)"))
			sz += 10
		}
		tmp[sz] = ' '
		sz++
		tmp[sz] = '|'
		sz++
		tmp[sz] = ' '
		sz++
	}

	buf.Write(tmp[:sz])
}

// AppendTimestamp appends a glog/klog-style timestamp to the given slice,
// returning how many bytes were written.
//
// Pre: len(buf) >= 20
func AppendTimestamp(buf []byte) int {

	// Avoid Fprintf, for speed. The format is so simple that we can do it quickly by hand.
	// It's worth about 3X. Fprintf is hard.
	now := time.Now()
	_, month, day := now.Date()
	hour, min, sec := now.Clock()

	// mmdd hh:mm:ss.uuuuuu
	sz := 0
	buf[sz+0] = '0' + byte(month)/10
	buf[sz+1] = '0' + byte(month)%10
	buf[sz+2] = '0' + byte(day/10)
	buf[sz+3] = '0' + byte(day%10)
	buf[sz+4] = ' '
	sz += 5
	buf[sz+0] = '0' + byte(hour/10)
	buf[sz+1] = '0' + byte(hour%10)
	buf[sz+2] = ':'
	buf[sz+3] = '0' + byte(min/10)
	buf[sz+4] = '0' + byte(min%10)
	buf[sz+5] = ':'
	buf[sz+6] = '0' + byte(sec/10)
	buf[sz+7] = '0' + byte(sec%10)
	buf[sz+8] = '.'
	sz += 9
	sz += AppendNDigits(6, now.Nanosecond()/1000, buf[sz:], '0')

	return sz
}

// AppendDigits appends the base 10 value to the given buffer, returning the number of bytes written.
//
// Pre: inValue > 0
func AppendDigits(inValue int, buf []byte) int {
	sz := 0
	for ; inValue > 0; sz++ {
		buf[sz] = '0' + byte(inValue%10)
		inValue /= 10
	}
	// Reverse the digits in place
	for i := sz/2 - 1; i >= 0; i-- {
		idx := sz - 1 - i
		tmp := buf[i]
		buf[i] = buf[idx]
		buf[idx] = tmp
	}
	return sz
}

// AppendNDigits formats an n-digit integer to the given buffer, padding as needed,
// returning the number of bytes written.
//
// Pre: len(buf) >= inNumDigits
func AppendNDigits(inNumDigits int, inValue int, buf []byte, inPad byte) int {
	j := inNumDigits - 1
	for ; j >= 0 && inValue > 0; j-- {
		buf[j] = '0' + byte(inValue%10)
		inValue /= 10
	}
	for ; j >= 0; j-- {
		buf[j] = inPad
	}
	return inNumDigits
}
