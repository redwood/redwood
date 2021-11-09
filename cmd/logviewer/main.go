package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

func matchesFilter(s string, filter []string) bool {
	var onePlus bool
	for _, f := range filter {
		if f[0] == '+' && strings.Contains(s, f[1:]) {
			onePlus = true
		} else if f[0] == '-' && strings.Contains(s, f[1:]) {
			return false
		}
	}
	return onePlus
}

func main() {
	leftFile := os.Args[1]
	rightFile := os.Args[2]
	filter := strings.Split(os.Args[3], ",")

	leftBytes, err := ioutil.ReadFile(leftFile)
	if err != nil {
		panic(err)
	}
	rightBytes, err := ioutil.ReadFile(rightFile)
	if err != nil {
		panic(err)
	}

	leftRaw := strings.Split(string(leftBytes), "\n")
	rightRaw := strings.Split(string(rightBytes), "\n")

	var firstTime time.Time

	var left []string
	var right []string
	var i, j int
	for {
		if i > len(leftRaw)-1 {
			break
		} else if j > len(rightRaw)-1 {
			break
		}

		if !matchesFilter(leftRaw[i], filter) {
			i++
			continue
		}
		if !matchesFilter(rightRaw[j], filter) {
			j++
			continue
		}

		leftTime, ok := parseTime(leftRaw[i])
		if !ok {
			i++
			continue
		}

		rightTime, ok := parseTime(rightRaw[j])
		if !ok {
			j++
			continue
		}

		if firstTime.IsZero() {
			if leftTime.Before(rightTime) {
				firstTime = leftTime
			} else {
				firstTime = rightTime
			}
		}

		if leftTime.Before(rightTime) {
			seconds := leftTime.Sub(firstTime).Round(1 * time.Second).Seconds()

			idx := strings.IndexByte(leftRaw[i], ']') + 1

			left = append(left, fmt.Sprintf("%3.0f %v", seconds, leftRaw[i][idx:]))
			right = append(right, " ")
			i++

		} else {
			seconds := rightTime.Sub(firstTime).Round(1 * time.Second).Seconds()

			idx := strings.IndexByte(rightRaw[j], ']') + 1

			left = append(left, fmt.Sprintf("%3.0f", seconds))
			right = append(right, fmt.Sprintf("%v", rightRaw[j][idx:]))
			j++
		}
	}

	uiState.logLinesLeft = left
	uiState.logLinesRight = right

	tui := NewTermUI()
	tui.Start()
}

var timeRegexp = regexp.MustCompile(`(\d\d):(\d\d):(\d\d)\.(\d\d\d\d\d\d)`)

func parseTime(timeStr string) (time.Time, bool) {
	parts := timeRegexp.FindStringSubmatch(timeStr)
	if len(parts) < 5 {
		return time.Time{}, false
	}
	hStr := parts[1]
	mStr := parts[2]
	sStr := parts[3]
	nsStr := parts[4]

	h, err := strconv.ParseUint(hStr, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	m, err := strconv.ParseUint(mStr, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	s, err := strconv.ParseUint(sStr, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	ns, err := strconv.ParseUint(nsStr, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse("15:04:05", fmt.Sprintf("%02d:%02d:%02d", h, m, s))
	if err != nil {
		panic(err)
		return time.Time{}, false
	}
	return t.Add(time.Duration(ns) * time.Nanosecond), true
}

type termUI struct {
	LeftPane  *logPane
	RightPane *logPane
	Input     *input
	Layout    *layout
	chDone    chan struct{}
	stopOnce  sync.Once
	focusMode focusMode
}

var uiState = struct {
	termWidth, termHeight int

	focusMode focusMode

	logLinesLeft       []string
	logLinesRight      []string
	logPanePreviousKey string
	logFilter          struct {
		debug   bool
		success bool
		info    bool
		warn    bool
		error   bool
	}
	logNeedsRescroll bool

	inputText     string
	cursorVisible bool
}{
	focusMode:          focusInput,
	logLinesLeft:       []string{},
	logLinesRight:      []string{},
	logPanePreviousKey: "",
	logFilter: struct {
		debug   bool
		success bool
		info    bool
		warn    bool
		error   bool
	}{true, true, true, true, true},
	inputText: "",
}

type focusMode int

const (
	focusInput focusMode = iota
	focusMain
	focusSidebar
)

func NewTermUI() *termUI {
	left := newLogPane(&uiState.logLinesLeft)
	right := newLogPane(&uiState.logLinesRight)
	input := newInput()
	return &termUI{
		LeftPane:  left,
		RightPane: right,
		Input:     input,
		Layout:    newLayout(left, right, input),
		chDone:    make(chan struct{}),
	}
}

func (tui *termUI) Start() {
	if err := ui.Init(); err != nil {
		panic(fmt.Sprintf("failed to initialize termui: %v", err))
	}
	defer ui.Close()
	defer tui.Stop()

	ui.Clear()

	uiState.termWidth, uiState.termHeight = ui.TerminalDimensions()
	tui.Layout.SetRect(0, 0, uiState.termWidth, uiState.termHeight)

	uiEvents := ui.PollEvents()

	shouldRender := true
Loop:
	for {
		if shouldRender || true {
			tui.Layout.Update()
			ui.Render(tui.LeftPane.Component(), tui.RightPane.Component(), tui.Layout.Component(), tui.Input.Component())
		}

		select {
		case <-tui.LeftPane.NeedsRefresh():
			shouldRender = true
			continue Loop
		case <-tui.RightPane.NeedsRefresh():
			shouldRender = true
			continue Loop
		case <-tui.Layout.NeedsRefresh():
			shouldRender = true
			continue Loop
		case <-tui.Input.NeedsRefresh():
			shouldRender = true
			continue Loop
		case evt := <-uiEvents:
			switch evt.Type {
			case ui.KeyboardEvent:
				switch evt.ID {
				case "<C-c>":
					return
				}
			case ui.ResizeEvent:
				uiState.termWidth, uiState.termHeight = ui.TerminalDimensions()
				tui.Layout.SetRect(0, 0, uiState.termWidth, uiState.termHeight)
				shouldRender = true
				continue Loop
			}
			handled := tui.Layout.HandleInput(evt)
			shouldRender = handled

		case <-tui.chDone:
			return
		}
	}
}

func (tui *termUI) Stop() {
	tui.stopOnce.Do(func() {
		close(tui.chDone)
		ui.Clear()
	})
}

func (tui *termUI) Done() <-chan struct{} {
	return tui.chDone
}

type layout struct {
	*ui.Grid
	*component

	leftPane  *logPane
	rightPane *logPane
	input     *input
}

func newLayout(leftPane *logPane, rightPane *logPane, input *input) *layout {
	grid := ui.NewGrid()
	l := &layout{
		component: nil,
		Grid:      grid,
		leftPane:  leftPane,
		rightPane: rightPane,
		input:     input,
	}
	l.component = newComponent(l)
	return l
}

func (l *layout) HandleInput(evt ui.Event) bool {
	switch evt.Type {
	case ui.KeyboardEvent:
		switch evt.ID {
		case "<C-i>":
			uiState.focusMode = focusInput
			return true
		case "<C-l>":
			uiState.focusMode = focusMain
			return true
		}
	}

	if uiState.focusMode == focusMain {
		return l.leftPane.HandleInput(evt) && l.rightPane.HandleInput(evt)
	} else if uiState.focusMode == focusInput {
		return l.input.HandleInput(evt)
	}

	return false
}

func (l *layout) Update() {
	width, height := uiState.termWidth, uiState.termHeight

	l.Grid.SetRect(0, 0, width, height-3)
	l.input.SetRect(0, height-3, width, height)

	l.leftPane.PaddingTop = 0
	l.leftPane.PaddingRight = 0
	l.leftPane.PaddingBottom = 0
	l.leftPane.PaddingLeft = 0
	l.leftPane.BorderTop = false
	l.leftPane.BorderRight = false
	l.leftPane.BorderBottom = false
	l.leftPane.BorderLeft = false

	l.rightPane.PaddingTop = 0
	l.rightPane.PaddingRight = 0
	l.rightPane.PaddingBottom = 0
	l.rightPane.PaddingLeft = 0
	l.rightPane.BorderTop = false
	l.rightPane.BorderRight = false
	l.rightPane.BorderBottom = false
	l.rightPane.BorderLeft = false

	l.input.PaddingTop = 0
	l.input.PaddingRight = 0
	l.input.PaddingBottom = 0
	l.input.PaddingLeft = 0
	l.input.BorderTop = false
	l.input.BorderRight = false
	l.input.BorderBottom = false
	l.input.BorderLeft = false

	l.Grid.Items = l.Grid.Items[:0]
	l.Grid.Set(
		ui.NewRow(1.0,
			ui.NewCol(0.5, l.leftPane.Component()),
			ui.NewCol(0.5, l.rightPane.Component()),
		),
	)

	l.leftPane.Update()
	l.rightPane.Update()
	l.input.Update()
}

type input struct {
	*component
	*widgets.Paragraph
}

func newInput() *input {
	para := widgets.NewParagraph()
	i := &input{
		component: newComponent(para),
		Paragraph: para,
	}

	go func() {
		for {
			<-time.After(500 * time.Millisecond)
			uiState.cursorVisible = !uiState.cursorVisible
			i.RequestRefresh()
		}
	}()
	return i
}

func (i *input) Update() {
	i.Paragraph.Text = "> " + uiState.inputText
	if uiState.focusMode == focusInput {
		i.Paragraph.BorderStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
		if uiState.cursorVisible {
			i.Paragraph.Text += "_"
		}
	} else {
		i.Paragraph.BorderStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierClear)
	}
}

func (i *input) HandleInput(evt ui.Event) bool {
	switch evt.Type {
	case ui.KeyboardEvent:
		switch evt.ID {
		case "<Backspace>":
			if len(uiState.inputText) > 0 {
				uiState.inputText = uiState.inputText[:len(uiState.inputText)-1]
			}
			return true

		case "<Enter>":
			if len(uiState.inputText) > 0 {
				i.processCommand(uiState.inputText)
				uiState.inputText = ""
			}
			return true

		case "<Space>":
			uiState.inputText += " "
			return true

		default:
			uiState.inputText += evt.ID
			return true
		}
	}
	return false
}

func (i *input) processCommand(cmd string) {
	parts := strings.Split(cmd, " ")

	switch parts[0] {
	case "log":
		switch parts[1] {
		case "toggle":
			switch parts[2] {
			case "info":
				uiState.logFilter.info = !uiState.logFilter.info
				uiState.logNeedsRescroll = true
			case "debug":
				uiState.logFilter.debug = !uiState.logFilter.debug
				uiState.logNeedsRescroll = true
			case "success":
				uiState.logFilter.success = !uiState.logFilter.success
				uiState.logNeedsRescroll = true
			case "warn":
				uiState.logFilter.warn = !uiState.logFilter.warn
				uiState.logNeedsRescroll = true
			case "error":
				uiState.logFilter.error = !uiState.logFilter.error
				uiState.logNeedsRescroll = true
			}
		}
	}
}

type logPane struct {
	termUI *termUI
	*widgets.List
	*component

	lines *[]string
}

func newLogPane(lines *[]string) *logPane {
	list := widgets.NewList()
	lp := &logPane{
		component: newComponent(list),
		List:      list,
		lines:     lines,
	}
	lp.List.SelectedRowStyle = ui.NewStyle(lp.List.SelectedRowStyle.Fg, lp.List.SelectedRowStyle.Bg, ui.ModifierReverse)
	return lp
}

func (p *logPane) scrollBottom() {
	p.SelectedRow = len(p.Rows) - 1
	if p.SelectedRow < 0 {
		p.SelectedRow = 0
	}
}

func (p *logPane) Update() {
	p.Lock()
	defer p.Unlock()

	lines := *p.lines

	var filteredLines []string
	for i := range lines {
		if len(lines[i]) == 0 {
			continue
		}
		switch lines[i][0] {
		case 'D':
			if uiState.logFilter.debug {
				filteredLines = append(filteredLines, lines[i][1:])
			}
		case 'S':
			if uiState.logFilter.success {
				filteredLines = append(filteredLines, lines[i][1:])
			}
		case 'I':
			if uiState.logFilter.info {
				filteredLines = append(filteredLines, lines[i][1:])
			}
		case 'W':
			if uiState.logFilter.warn {
				filteredLines = append(filteredLines, lines[i][1:])
			}
		case 'E':
			if uiState.logFilter.error {
				filteredLines = append(filteredLines, lines[i][1:])
			}
		default:
			filteredLines = append(filteredLines, lines[i])
		}
	}
	p.List.Rows = filteredLines

	if uiState.focusMode == focusMain {
		p.List.BorderStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	} else {
		p.List.BorderStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierClear)
	}
	// }

	if uiState.logNeedsRescroll {
		p.scrollBottom()
		uiState.logNeedsRescroll = false
	}
}

func (p *logPane) HandleInput(evt ui.Event) bool {
	p.Lock()
	defer p.Unlock()
	defer func() {
		if uiState.logPanePreviousKey == "g" {
			uiState.logPanePreviousKey = ""
		} else {
			uiState.logPanePreviousKey = evt.ID
		}
	}()
	switch evt.ID {
	case "j", "<Down>":
		p.ScrollDown()
		return true
	case "k", "<Up>":
		p.ScrollUp()
		return true
	case "<C-d>":
		p.ScrollHalfPageDown()
		return true
	case "<C-u>":
		p.ScrollHalfPageUp()
		return true
	case "<C-f>":
		p.ScrollPageDown()
		return true
	case "<C-b>":
		p.ScrollPageUp()
		return true
	case "g":
		if uiState.logPanePreviousKey == "g" {
			p.ScrollTop()
			return true
		}
	case "<Home>":
		p.ScrollTop()
		return true
	case "G", "<End>":
		p.ScrollBottom()
		return true
	}
	return false
}

type Component interface {
	ui.Drawable
	Update()
	HandleInput(evt ui.Event) bool
	Component() ui.Drawable
	NeedsRefresh() <-chan struct{}
}

type component struct {
	drawable     ui.Drawable
	needsRefresh chan struct{}
}

func newComponent(drawable ui.Drawable) *component {
	return &component{
		drawable:     drawable,
		needsRefresh: make(chan struct{}),
	}
}

func (c component) Component() ui.Drawable {
	return c.drawable
}

func (c component) Update() {}

func (c component) HandleInput(evt ui.Event) bool { return false }

func (c component) NeedsRefresh() <-chan struct{} {
	return c.needsRefresh
}

func (c component) RequestRefresh() {
	select {
	case c.needsRefresh <- struct{}{}:
	default:
	}
}
