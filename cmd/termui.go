package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type termUI struct {
	TabPane   *tabPane
	LogPane   *logPane
	StatePane *statePane
	Input     *input
	Sidebar   *sidebar
	Layout    *layout
	chDone    chan struct{}
	stopOnce  sync.Once
	focusMode focusMode
}

var uiState = struct {
	termWidth, termHeight int

	activeTab tab
	focusMode focusMode

	logLines           []string
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

	stateURIs         []string
	states            map[string]string
	selectedState     int
	previousTreeState interface{}
}{
	activeTab:          tabLogs,
	focusMode:          focusInput,
	logLines:           []string{},
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

type tab int

const (
	tabLogs tab = iota
	tabStates
)

type focusMode int

const (
	focusInput focusMode = iota
	focusMain
	focusSidebar
)

func NewTermUI() *termUI {
	tabPane := newTabPane()
	logPane := newLogPane(30000)
	statePane := newStatePane()
	input := newInput()
	sidebar := newSidebar()
	return &termUI{
		TabPane:   tabPane,
		LogPane:   logPane,
		StatePane: statePane,
		Input:     input,
		Sidebar:   sidebar,
		Layout:    newLayout(tabPane, logPane, statePane, input, sidebar),
		chDone:    make(chan struct{}),
	}
}

func (tui *termUI) Start() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
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
			ui.Render(tui.TabPane.Component(), tui.Layout.Component(), tui.Input.Component())
		}

		select {
		case <-tui.LogPane.NeedsRefresh():
			shouldRender = true
			continue Loop
		case <-tui.Layout.NeedsRefresh():
			shouldRender = true
			continue Loop
		case <-tui.Input.NeedsRefresh():
			shouldRender = true
			continue Loop
		case <-tui.Sidebar.NeedsRefresh():
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
	*component
	*ui.Grid

	tabPane   *tabPane
	logPane   *logPane
	statePane *statePane
	input     *input
	sidebar   *sidebar
}

func newLayout(tabPane *tabPane, logPane *logPane, statePane *statePane, input *input, sidebar *sidebar) *layout {
	grid := ui.NewGrid()
	l := &layout{
		component: nil,
		Grid:      grid,
		tabPane:   tabPane,
		logPane:   logPane,
		statePane: statePane,
		input:     input,
		sidebar:   sidebar,
	}
	l.component = newComponent(l)
	return l
}

func (l *layout) HandleInput(evt ui.Event) bool {
	switch evt.Type {
	case ui.KeyboardEvent:
		switch evt.ID {
		case "<C-p>":
			uiState.focusMode = focusInput
			return true
		case "<C-l>":
			uiState.focusMode = focusMain
			return true
		case "<C-s>":
			uiState.focusMode = focusSidebar
			return true
		}
	}

	handled := l.tabPane.HandleInput(evt)
	if handled {
		return true
	}

	if uiState.focusMode == focusMain {
		if uiState.activeTab == tabLogs {
			return l.logPane.HandleInput(evt)
		} else if uiState.activeTab == tabStates {
			return l.statePane.HandleInput(evt)
		}
	} else if uiState.focusMode == focusInput {
		return l.input.HandleInput(evt)
	} else if uiState.focusMode == focusSidebar {
		return l.sidebar.HandleInput(evt)
	}

	return false
}

func (l *layout) Update() {
	width, height := uiState.termWidth, uiState.termHeight

	l.tabPane.SetRect(0, 0, width, 2)
	l.Grid.SetRect(0, 2, width, height-3)
	l.input.SetRect(0, height-3, width, height)

	l.tabPane.PaddingTop = 0
	l.tabPane.PaddingRight = 0
	l.tabPane.PaddingBottom = 0
	l.tabPane.PaddingLeft = 0
	l.tabPane.BorderTop = false
	l.tabPane.BorderRight = false
	l.tabPane.BorderBottom = false
	l.tabPane.BorderLeft = false

	l.Grid.PaddingTop = 0
	l.Grid.PaddingRight = 0
	l.Grid.PaddingBottom = 0
	l.Grid.PaddingLeft = 0

	l.input.PaddingTop = 0
	l.input.PaddingRight = 0
	l.input.PaddingBottom = 0
	l.input.PaddingLeft = 0
	// l.input.BorderTop = false
	// l.input.BorderRight = false
	// l.input.BorderBottom = false
	// l.input.BorderLeft = false

	var mainContent Component
	switch uiState.activeTab {
	case tabLogs:
		mainContent = l.logPane
	case tabStates:
		mainContent = l.statePane
	}

	l.Grid.Items = l.Grid.Items[:0]
	l.Grid.Set(
		ui.NewRow(1.0,
			ui.NewCol(0.8, mainContent.Component()),
			ui.NewCol(0.2, l.sidebar.Component()),
		),
	)

	l.tabPane.Update()
	l.logPane.Update()
	l.statePane.Update()
	l.input.Update()
	l.sidebar.Update()
}

type statePane struct {
	*component
	*widgets.Tree
}

func newStatePane() *statePane {
	tree := widgets.NewTree()
	tree.SelectedRowStyle = ui.NewStyle(tree.SelectedRowStyle.Fg, tree.SelectedRowStyle.Bg, ui.ModifierReverse)
	return &statePane{
		component: newComponent(tree),
		Tree:      tree,
	}
}

type stringer string

func (s stringer) String() string {
	return string(s)
}

func (p *statePane) Update() {
	if uiState.focusMode == focusMain {
		p.Tree.BorderStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	} else {
		p.Tree.BorderStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierClear)
	}

	if len(uiState.stateURIs) == 0 || len(uiState.states) == 0 {
		return
	}

	stateURI := uiState.stateURIs[uiState.selectedState]
	state := uiState.states[stateURI]

	var m map[string]interface{}
	_ = json.Unmarshal([]byte(state), &m)
	if reflect.DeepEqual(m, uiState.previousTreeState) {
		return
	}
	uiState.previousTreeState = m

	var walk func(val interface{}) []*widgets.TreeNode
	walk = func(val interface{}) []*widgets.TreeNode {
		switch val := val.(type) {
		case map[string]interface{}:
			var sorted sort.StringSlice
			for k := range val {
				sorted = append(sorted, k)
			}
			sort.Sort(sorted)

			var nodes []*widgets.TreeNode
			for _, k := range sorted {
				switch v := val[k].(type) {
				case map[string]interface{}, []interface{}:
					nodes = append(nodes, &widgets.TreeNode{Value: stringer(k), Nodes: walk(v)})
				case string, uint64, int64, float64:
					nodes = append(nodes, &widgets.TreeNode{Value: stringer(fmt.Sprintf("%v: %v", k, v)), Nodes: nil})
				}
			}
			return nodes

		case []interface{}:
			var nodes []*widgets.TreeNode
			for i, v := range val {
				switch v := v.(type) {
				case map[string]interface{}, []interface{}:
					nodes = append(nodes, &widgets.TreeNode{Value: stringer(fmt.Sprintf("%v", i)), Nodes: walk(v)})
				case string, uint64, int64, float64:
					nodes = append(nodes, &widgets.TreeNode{Value: stringer(fmt.Sprintf("%v: %v", i, v)), Nodes: nil})
				}
			}
			return nodes

		case string:
			return []*widgets.TreeNode{{Value: stringer(val), Nodes: nil}}
		case uint64:
			return []*widgets.TreeNode{{Value: stringer(fmt.Sprintf("%v", val)), Nodes: nil}}
		case int64:
			return []*widgets.TreeNode{{Value: stringer(fmt.Sprintf("%v", val)), Nodes: nil}}
		case float64:
			return []*widgets.TreeNode{{Value: stringer(fmt.Sprintf("%v", val)), Nodes: nil}}
		}
		return nil
	}
	nodes := walk(m)

	p.Tree.SetNodes(nodes)
}

func (p *statePane) HandleInput(evt ui.Event) bool {
	defer func() {
		if uiState.logPanePreviousKey == "g" {
			uiState.logPanePreviousKey = ""
		} else {
			uiState.logPanePreviousKey = evt.ID
		}
	}()
	switch evt.ID {
	case "<Enter>":
		p.ToggleExpand()
		return true
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

func (p *statePane) SetStates(states map[string]string) {
	uiState.states = states
	p.RequestRefresh()
}

type tabPane struct {
	*component
	*widgets.TabPane
}

func newTabPane() *tabPane {
	tabpane := widgets.NewTabPane(" Logs ", " State ", " Network ")
	tabpane.Border = false
	tabpane.ActiveTabStyle = ui.NewStyle(ui.ColorWhite, ui.ColorRed)
	return &tabPane{
		component: newComponent(tabpane),
		TabPane:   tabpane,
	}
}

func (p *tabPane) HandleInput(evt ui.Event) bool {
	switch evt.Type {
	case ui.KeyboardEvent:
		switch evt.ID {
		case "<F1>":
			uiState.activeTab = tabLogs
			return true
		case "<F2>":
			uiState.activeTab = tabStates
			return true
		}
	}
	return false
}

func (p *tabPane) Update() {
	p.TabPane.ActiveTabIndex = int(uiState.activeTab)
}

type sidebar struct {
	*component
	*widgets.List
}

func newSidebar() *sidebar {
	list := widgets.NewList()
	list.BorderStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierClear)
	list.SelectedRowStyle = ui.NewStyle(list.SelectedRowStyle.Fg, list.SelectedRowStyle.Bg, ui.ModifierReverse)
	return &sidebar{
		component: newComponent(list),
		List:      list,
	}
}

func (s *sidebar) Update() {
	if uiState.focusMode == focusSidebar {
		s.List.BorderStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	} else {
		s.List.BorderStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierClear)
	}
	s.List.Rows = uiState.stateURIs
}

func (s *sidebar) HandleInput(evt ui.Event) bool {
	defer func() {
		if uiState.logPanePreviousKey == "g" {
			uiState.logPanePreviousKey = ""
		} else {
			uiState.logPanePreviousKey = evt.ID
		}
	}()
	switch evt.ID {
	case "<Enter>":
		uiState.selectedState = s.List.SelectedRow
		return true
	case "j", "<Down>":
		s.ScrollDown()
		return true
	case "k", "<Up>":
		s.ScrollUp()
		return true
	case "<C-d>":
		s.ScrollHalfPageDown()
		return true
	case "<C-u>":
		s.ScrollHalfPageUp()
		return true
	case "<C-f>":
		s.ScrollPageDown()
		return true
	case "<C-b>":
		s.ScrollPageUp()
		return true
	case "g":
		if uiState.logPanePreviousKey == "g" {
			s.ScrollTop()
			return true
		}
	case "<Home>":
		s.ScrollTop()
		return true
	case "G", "<End>":
		s.ScrollBottom()
		return true
	}
	return false
}

func (s *sidebar) SetStateURIs(stateURIs []string) {
	uiState.stateURIs = stateURIs
	s.RequestRefresh()
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
	*widgets.List
	*component
	capacity int
}

func newLogPane(capacity int) *logPane {
	list := widgets.NewList()
	lp := &logPane{
		component: newComponent(list),
		List:      list,
		capacity:  capacity,
	}
	lp.List.SelectedRowStyle = ui.NewStyle(lp.List.SelectedRowStyle.Fg, lp.List.SelectedRowStyle.Bg, ui.ModifierReverse)
	return lp
}

func (p *logPane) Write(bs []byte) (int, error) {
	p.Lock()
	defer p.Unlock()

	bs = bytes.TrimSpace(bs)
	lines := strings.Split(string(bs), "\n")

	for i := range lines {
		idx := strings.IndexByte(lines[i], '|')
		if idx >= 0 {
			lines[i] = lines[i][:idx+1] + "[" + lines[i][idx+1:] + "](fg:clear)"
		}
	}

	uiState.logLines = append(uiState.logLines, lines...)
	if len(uiState.logLines) > p.capacity {
		uiState.logLines = uiState.logLines[len(uiState.logLines)-p.capacity:]
	}

	p.scrollBottom()
	p.RequestRefresh()

	return len(bs), nil
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

	var filteredLines []string
	for i := range uiState.logLines {
		switch uiState.logLines[i][0] {
		case 'D':
			if uiState.logFilter.debug {
				filteredLines = append(filteredLines, uiState.logLines[i][1:])
			}
		case 'S':
			if uiState.logFilter.success {
				filteredLines = append(filteredLines, uiState.logLines[i][1:])
			}
		case 'I':
			if uiState.logFilter.info {
				filteredLines = append(filteredLines, uiState.logLines[i][1:])
			}
		case 'W':
			if uiState.logFilter.warn {
				filteredLines = append(filteredLines, uiState.logLines[i][1:])
			}
		case 'E':
			if uiState.logFilter.error {
				filteredLines = append(filteredLines, uiState.logLines[i][1:])
			}
		default:
			filteredLines = append(filteredLines, uiState.logLines[i])
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
