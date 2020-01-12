package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/encoding"
	"github.com/nbedos/cistern/providers"
	"github.com/nbedos/cistern/tui"
	"github.com/nbedos/cistern/utils"
)

type focus int

const (
	table focus = iota
	search
	ref
	help
)

type keyBinding struct {
	keys   []string
	action string
}

var tableKeyBindings = []keyBinding{
	{
		keys:   []string{"Up", "k", "Ctrl-p"},
		action: "Move cursor up by one line",
	},
	{
		keys:   []string{"Down", "j", "Ctrl-n"},
		action: "Move cursor down by one line",
	},
	{
		keys:   []string{"Right", "l"},
		action: "Scroll right",
	},
	{
		keys:   []string{"Left", "h"},
		action: "Scroll left",
	},
	{
		keys:   []string{"Ctrl-u"},
		action: "Move cursor up by half a page",
	},
	{
		keys:   []string{"Page Up", "Ctrl-B"},
		action: "Move cursor up by one page",
	},
	{
		keys:   []string{"Ctrl-d"},
		action: "Move cursor down by half a page",
	},
	{
		keys:   []string{"Page Down", "Ctrl-F"},
		action: "Move cursor down by one page",
	},
	{
		keys:   []string{"Home"},
		action: "Move cursor to the first line",
	},
	{
		keys:   []string{"End"},
		action: "Move cursor to the last line",
	},
	{
		keys:   []string{"<"},
		action: "Move sort column left",
	},
	{
		keys:   []string{">"},
		action: "Move sort column right",
	},
	{
		keys:   []string{"!"},
		action: "Reverse sort order",
	},
	{
		keys:   []string{"o", "+"},
		action: "Open the fold at the cursor",
	},
	{
		keys:   []string{"O"},
		action: "Open the fold at the cursor and all sub-folds",
	},
	{
		keys:   []string{"c", "-"},
		action: "Close the fold at the cursor",
	},
	{
		keys:   []string{"C"},
		action: "Close the fold at the cursor and all sub-folds",
	},
	{
		keys:   []string{"b"},
		action: "Open associated web page in $BROWSER",
	},
	{
		keys:   []string{"v"},
		action: "View the log of the job at the cursor",
	},
	{
		keys:   []string{"/"},
		action: "Open search prompt",
	},
	{
		keys:   []string{"Escape"},
		action: "Close search prompt",
	},
	{
		keys:   []string{"Enter", "n"},
		action: "Move to the next match",
	},
	{
		keys:   []string{"N"},
		action: "Move to the previous match",
	},
	{
		keys:   []string{"g"},
		action: "Open git reference selection prompt",
	},
	{
		keys:   []string{"?"},
		action: "Show help screen",
	},
	{
		keys:   []string{"q"},
		action: "Quit",
	},
}

var shortTableKeyBindings = []keyBinding{
	{
		keys:   []string{"j"},
		action: "Down",
	},
	{
		keys:   []string{"k"},
		action: "Up",
	},
	{
		keys:   []string{"oO"},
		action: "Open",
	},
	{
		keys:   []string{"cC"},
		action: "Close",
	},
	{
		keys:   []string{"/"},
		action: "Search",
	},
	{
		keys:   []string{"v"},
		action: "Logs",
	},
	{
		keys:   []string{"b"},
		action: "Browser",
	},
	{
		keys:   []string{"?"},
		action: "Help",
	},
	{
		keys:   []string{"q"},
		action: "Quit",
	},
}

var searchKeyBindings = []keyBinding{
	{
		keys:   []string{"Enter"},
		action: "Search",
	},
	{
		keys:   []string{"Backspace"},
		action: "Delete last character",
	},
	{
		keys:   []string{"Ctrl-U"},
		action: "Delete whole line",
	},
	{
		keys:   []string{"Escape"},
		action: "Close prompt",
	},
}

var shortSearchKeyBindings = []keyBinding{
	{
		keys:   []string{"Enter"},
		action: "Search",
	},
	{
		keys:   []string{"Backspace"},
		action: "Delete character",
	},
	{
		keys:   []string{"Ctrl-U"},
		action: "Delete line",
	},
	{
		keys:   []string{"Escape"},
		action: "Abort",
	},
}

var refKeyBindings = []keyBinding{
	{
		keys:   []string{"Enter"},
		action: "Validate",
	},
	{
		keys:   []string{"Backspace"},
		action: "Delete last character",
	},
	{
		keys:   []string{"Ctrl-U"},
		action: "Delete whole line",
	},
	{
		keys:   []string{"Tab", "Shift-Tab"},
		action: "Complete",
	},
	{
		keys:   []string{"Up", "Ctrl-P"},
		action: "Move the cursor to the previous suggestion",
	},
	{
		keys:   []string{"Down", "Ctrl-N"},
		action: "Move the cursor to the next suggestion",
	},
	{
		keys:   []string{"Page up", "Ctrl-B"},
		action: "Move the cursor up by one page",
	},
	{
		keys:   []string{"Page down", "Ctrl-F"},
		action: "Move the cursor down by one page",
	},
	{
		keys:   []string{"Escape"},
		action: "Close prompt",
	},
}

var shortRefKeyBindings = []keyBinding{
	{
		keys:   []string{"Enter"},
		action: "Validate",
	},
	{
		keys:   []string{"Tab", "Shift-Tab"},
		action: "Complete",
	},
	{
		keys:   []string{"Up"},
		action: "Up",
	},
	{
		keys:   []string{"Down"},
		action: "Down",
	},
	{
		keys:   []string{"Escape"},
		action: "Abort",
	},
}

var shortHelpKeyBindings = []keyBinding{
	{
		keys:   []string{"j"},
		action: "Up",
	},
	{
		keys:   []string{"k"},
		action: "Down",
	},
	{
		keys:   []string{"Ctrl-B"},
		action: "Page up",
	},
	{
		keys:   []string{"Ctrl-F"},
		action: "Page down",
	},
	{
		keys:   []string{"q"},
		action: "Quit",
	},
}

var helpKeyBindings = []keyBinding{
	{
		keys:   []string{"j", "Down", "Ctrl-N"},
		action: "Scroll down by one line",
	},
	{
		keys:   []string{"k", "Up", "Ctrl-P"},
		action: "Scroll up by one line",
	},
	{
		keys:   []string{"Page up", "Ctrl-B"},
		action: "Scroll up by one page",
	},
	{
		keys:   []string{"Page down", "Ctrl-F"},
		action: "Scroll down by one page",
	},
	{
		keys:   []string{"Ctrl-U"},
		action: "Scroll up by half a page",
	},
	{
		keys:   []string{"Ctrl-D"},
		action: "Scroll down by half a page",
	},
	{
		keys:   []string{"q"},
		action: "Exit help screen",
	},
}

func helpScreen(emphasis tui.StyleTransform) []tui.StyledString {
	draw := func(bindings []keyBinding) []tui.StyledString {
		lines := make([]tui.StyledString, 0)
		for _, b := range bindings {
			keys := make([]tui.StyledString, 0)
			for _, k := range b.keys {
				keys = append(keys, tui.NewStyledString(k, emphasis))
			}
			line := tui.NewStyledString("   ")
			line.AppendString(tui.Join(keys, tui.NewStyledString(", ")))
			line.Fit(tui.Left, 25)
			line.Append(b.action)
			lines = append(lines, line)
		}
		return lines
	}

	ss := []tui.StyledString{
		tui.NewStyledString("HELP FOR INTERACTIVE COMMANDS", emphasis),
		{},
		{},
	}

	ss = append(ss, tui.NewStyledString("Tabular view:", emphasis))
	ss = append(ss, tui.StyledString{})
	ss = append(ss, draw(tableKeyBindings)...)
	ss = append(ss, tui.StyledString{}, tui.StyledString{})

	ss = append(ss, tui.NewStyledString("Search prompt:", emphasis))
	ss = append(ss, tui.StyledString{})
	ss = append(ss, draw(searchKeyBindings)...)
	ss = append(ss, tui.StyledString{}, tui.StyledString{})

	ss = append(ss, tui.NewStyledString("Git reference selection prompt:", emphasis))
	ss = append(ss, tui.StyledString{})
	ss = append(ss, draw(refKeyBindings)...)
	ss = append(ss, tui.StyledString{}, tui.StyledString{})

	ss = append(ss, tui.NewStyledString("Help screen", emphasis))
	ss = append(ss, tui.StyledString{})
	ss = append(ss, draw(helpKeyBindings)...)
	ss = append(ss, tui.StyledString{}, tui.StyledString{})

	return ss
}

func (c *Controller) shortKeyBindings() tui.StyledString {
	var bindings []keyBinding
	switch c.focus {
	case table:
		bindings = shortTableKeyBindings
	case search:
		bindings = shortSearchKeyBindings
	case ref:
		bindings = shortRefKeyBindings
	case help:
		bindings = shortHelpKeyBindings
	}

	s := tui.StyledString{}
	for i, b := range bindings {
		if i > 0 {
			s.Append("  ")
		}
		s.Append(strings.Join(b.keys, "/") + ":")
		s.Append(b.action)
	}
	s.Fit(tui.Left, c.width)
	s.Apply(func(s tcell.Style) tcell.Style {
		return s.Reverse(true)
	})

	return s
}

type WindowDimensions struct {
	x      int
	y      int
	width  int
	height int
}

type ControllerConfiguration struct {
	tui.TableConfiguration
	providers.GitStyle
}

type Controller struct {
	tui         *tui.TUI
	cache       providers.Cache
	ref         string
	width       int
	height      int
	header      *tui.TextArea
	table       *tui.HierarchicalTable
	tableSearch string
	status      *tui.TextArea
	refcmd      *tui.Command
	searchcmd   *tui.Command
	keyhints    *tui.TextArea
	focus       focus
	help        *tui.TextArea
	layout      map[tui.Widget]WindowDimensions
	style       providers.GitStyle
}

var ErrExit = errors.New("exit")

func NewController(ui *tui.TUI, conf ControllerConfiguration, ref string, c providers.Cache) (Controller, error) {
	// Arbitrary values, the correct size will be set when the first RESIZE event is received
	width, height := ui.Size()
	header, err := tui.NewTextArea(width, height)
	if err != nil {
		return Controller{}, err
	}

	table, err := tui.NewHierarchicalTable(conf.TableConfiguration, nil, width, height)
	if err != nil {
		return Controller{}, err
	}

	status, err := tui.NewTextArea(width, height)
	if err != nil {
		return Controller{}, err
	}

	keyhints, err := tui.NewTextArea(width, height)
	if err != nil {
		return Controller{}, err
	}

	search := tui.NewCommand(width, height, "Search: ")
	command := tui.NewCommand(width, height, "Ref: ")

	help, err := tui.NewTextArea(width, height)
	if err != nil {
		return Controller{}, err
	}
	bold := func(s tcell.Style) tcell.Style { return s.Bold(true) }
	help.WriteContent(helpScreen(bold)...)

	return Controller{
		tui:       ui,
		ref:       ref,
		cache:     c,
		width:     width,
		height:    height,
		header:    &header,
		table:     &table,
		status:    &status,
		searchcmd: &search,
		refcmd:    &command,
		keyhints:  &keyhints,
		help:      &help,
		style:     conf.GitStyle,
		layout:    make(map[tui.Widget]WindowDimensions),
	}, nil
}

func (c *Controller) setRef(ref string) {
	pipelines := make([]tui.TableNode, 0)
	for _, pipeline := range c.cache.Pipelines(c.ref) {
		pipelines = append(pipelines, pipeline)
	}
	c.table.Replace(pipelines)
	c.ref = ref
}

func (c *Controller) focusWidget(f focus) {
	c.focus = f
	switch f {
	case table:
	case ref:
	case search:
	case help:
	}
}

func (c *Controller) Run(ctx context.Context, repositoryURL string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errc := make(chan error)
	refc := make(chan string)
	updates := make(chan time.Time)

	c.writeStatus("")
	c.refresh()
	c.draw()

	// Start pipeline monitoring
	go func() {
		select {
		case refc <- c.ref:
		case <-ctx.Done():
		}
	}()

	var tmpRef string
	var mux = &sync.Mutex{}
	var refCtx context.Context
	var refCancel = func() {}
	var err error
	for err == nil {
		select {
		case ref := <-refc:
			// Each time a new git reference is received, cancel the last function call
			// and start a new one.
			mux.Lock()
			tmpRef = ref
			mux.Unlock()
			refCancel()
			refCtx, refCancel = context.WithCancel(ctx)
			go func(ctx context.Context, refName string) {
				var err error
				var repositoryURLs []string
				ref := providers.Ref{
					Name: refName,
				}
				repositoryURLs, ref.Commit, err = providers.RemotesAndCommit(repositoryURL, ref.Name)
				switch err {
				case providers.ErrUnknownRepositoryURL:
					// The path does not refer to a local repository so it probably is a url
					repositoryURLs = []string{repositoryURL}
				case nil:
					suggestions, err := providers.References(repositoryURL, c.style)
					if err != nil {
						errc <- err
						return
					}
					c.refcmd.SetCompletions(suggestions)
				default:
					errc <- err
					return
				}

				errc <- c.cache.MonitorPipelines(ctx, repositoryURLs, ref, updates)
			}(refCtx, ref)

		case <-updates:
			// Update the controller once we receive an update, meaning the reference exists at
			// least locally or remotely
			mux.Lock()
			c.setRef(tmpRef)
			mux.Unlock()
			c.refresh()
			c.draw()

		case e := <-errc:
			switch e {
			case context.Canceled:
				// Do nothing
			case providers.ErrUnknownGitReference:
				c.writeStatus("error: git reference was not found on remote server(s)")
				c.draw()
			default:
				err = e
			}

		case event := <-c.tui.Eventc:
			err = c.process(ctx, event, refc)

		case <-ctx.Done():
			err = ctx.Err()
		}
	}

	if err == ErrExit {
		return nil
	}
	return err
}

func (c *Controller) SetHeader(lines []tui.StyledString) {
	c.header.WriteContent(lines...)
}

func (c *Controller) writeStatus(s string) {
	msg := tui.NewStyledString(s)
	msg.Fit(tui.Left, c.width)
	//msg.Apply(func(s tcell.Style) tcell.Style {
	//	return s.Reverse(true)
	//})
	c.status.WriteContent(msg)
}

func (c *Controller) refresh() {
	commit, _ := c.cache.Commit(c.ref)
	c.header.WriteContent(commit.StyledStrings(c.style)...)
	pipelines := make([]tui.TableNode, 0)
	for _, pipeline := range c.cache.Pipelines(c.ref) {
		pipelines = append(pipelines, pipeline)
	}
	c.table.Replace(pipelines)
	c.resize(c.width, c.height)
}

func (c *Controller) nextMatch(ascending bool) {
	if c.tableSearch != "" {
		found := c.table.ScrollToNextMatch(c.tableSearch, ascending)
		if !found {
			c.writeStatus(fmt.Sprintf("No match found for %#v", c.tableSearch))
		}
	}
}

func (c *Controller) resize(width int, height int) {
	c.width = utils.MaxInt(width, 0)
	c.height = utils.MaxInt(height, 0)

	c.layout[c.help] = WindowDimensions{
		width:  c.width,
		height: utils.MaxInt(0, c.height-1),
	}

	c.layout[c.header] = WindowDimensions{
		width:  c.width,
		height: utils.MinInt(utils.MinInt(len(c.header.Content)+2, 9), c.height),
	}
	y := c.layout[c.header].height

	c.layout[c.table] = WindowDimensions{
		y:      y,
		width:  c.width,
		height: utils.MaxInt(0, c.height-c.layout[c.header].height-2),
	}
	y += c.layout[c.table].height

	c.layout[c.status] = WindowDimensions{
		y:      y,
		width:  c.width,
		height: 1,
	}

	c.layout[c.searchcmd] = WindowDimensions{
		y:      y,
		width:  c.width,
		height: 1,
	}

	c.layout[c.refcmd] = WindowDimensions{
		y:      y - utils.MinInt(14, y) + 1,
		width:  c.width,
		height: utils.MinInt(14, y),
	}
	y += 1

	c.layout[c.keyhints] = WindowDimensions{
		y:      y,
		width:  c.width,
		height: 1,
	}

	for w, dim := range c.layout {
		w.Resize(dim.width, dim.height)
	}

	if len(c.status.Content) > 0 {
		// Call writeStatus since the result depends on the c.width
		c.writeStatus(c.status.Content[0].String())
	}
}

// Turn `aaa\rbbb\rccc\r\n` into `ccc\r\n`
// This is mostly for Travis logs that contain metadata hidden by carriage returns
var deleteUntilCarriageReturn = regexp.MustCompile(`.*\r([^\r\n])`)

// https://stackoverflow.com/questions/14693701/how-can-i-remove-the-ansi-escape-sequences-from-a-string-in-python
var deleteANSIEscapeSequence = regexp.MustCompile(`\x1b[@-_][0-?]*[ -/]*[@-~]`)

func (c *Controller) viewLog(ctx context.Context) error {
	c.writeStatus("Fetching logs...")
	c.draw()
	defer func() {
		c.draw()
	}()
	key, ids, exists := c.activeStepPath()
	if !exists {
		return providers.ErrNoLogHere
	}

	log, err := c.cache.Log(ctx, key, ids)
	if err != nil {
		if err == providers.ErrNoLogHere {
			return nil
		}
		return err
	}

	stdin := bytes.Buffer{}
	log = deleteANSIEscapeSequence.ReplaceAllString(log, "")
	log = deleteUntilCarriageReturn.ReplaceAllString(log, "$1")
	stdin.WriteString(log)

	// FIXME Do not make this choice here, move this to the configuration
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	return c.tui.Exec(ctx, pager, nil, &stdin)
}

func (c Controller) activeStepPath() (providers.PipelineKey, []string, bool) {
	if stepPath := c.table.ActiveNodePath(); len(stepPath) > 0 {
		key, ok := stepPath[0].(providers.PipelineKey)
		if !ok {
			return providers.PipelineKey{}, nil, false
		}

		stepIDs := make([]string, 0)
		for _, id := range stepPath[1:] {
			s, ok := id.(string)
			if !ok {
				return providers.PipelineKey{}, nil, false
			}
			stepIDs = append(stepIDs, s)
		}

		return key, stepIDs, true
	}

	return providers.PipelineKey{}, nil, false
}

func (c Controller) openActiveRowInBrowser() error {
	if key, ids, exists := c.activeStepPath(); exists {
		step, exists := c.cache.Step(key, ids)
		if exists && step.WebURL.Valid {
			// TODO Move this to configuration file
			browser := os.Getenv("BROWSER")
			if browser == "" {
				return errors.New(fmt.Sprintf("BROWSER environment variable not set. You can instead open %s in your browser.", step.WebURL.String))
			}

			return exec.Command(browser, step.WebURL.String).Start()
		}
	}

	return nil
}

func (c *Controller) draw() {
	c.tui.Clear()
	widgets := make([]tui.Widget, 0)
	if c.focus == help {
		widgets = append(widgets, c.help)
	} else {
		widgets = append(widgets, c.header, c.table)
		switch c.focus {
		case ref:
			widgets = append(widgets, c.refcmd)
		case search:
			widgets = append(widgets, c.searchcmd)
		default:
			widgets = append(widgets, c.status)
		}
	}
	widgets = append(widgets, c.keyhints)

	c.keyhints.WriteContent(c.shortKeyBindings())

	for _, widget := range widgets {
		dim := c.layout[widget]
		window := c.tui.Window(dim.x, dim.y, dim.width, dim.height)
		widget.Draw(window)
	}

	c.tui.Show()
}

func (c *Controller) process(ctx context.Context, event tcell.Event, refc chan<- string) error {
	c.writeStatus("")

	switch ev := event.(type) {
	case *tcell.EventResize:
		sx, sy := ev.Size()
		c.resize(sx, sy)
	case *tcell.EventKey:
		switch c.focus {
		case help:
			if ev.Key() == tcell.KeyRune && ev.Rune() == 'q' {
				c.focus = table
			} else {
				c.help.Process(ev)
			}
		case ref:
			if ev.Key() == tcell.KeyEnter {
				if ref := c.refcmd.Input(); ref != "" {
					go func() {
						select {
						case refc <- ref:
							// Do nothing
						case <-ctx.Done():
							return
						}
					}()
				}
				c.focus = table
			} else {
				c.refcmd.Process(ev)
				if ev.Key() == tcell.KeyEsc {
					c.focus = table
				}
			}

		case search:
			if ev.Key() == tcell.KeyEnter {
				c.tableSearch = c.searchcmd.Input()
				c.nextMatch(true)
				c.focus = table
			} else {
				c.searchcmd.Process(ev)
				if ev.Key() == tcell.KeyEsc {
					c.focus = table
				}
			}

		case table:
			if ev.Key() == tcell.KeyRune {
				switch keyRune := ev.Rune(); keyRune {
				case 'b':
					if err := c.openActiveRowInBrowser(); err != nil {
						return err
					}
				case 'g':
					c.focus = ref
					c.refcmd.Focus()
				case 'n', 'N':
					c.nextMatch(ev.Rune() == 'n')
				case 'q':
					return ErrExit
				case '/':
					c.focus = search
					c.searchcmd.Focus()
				case 'u':
					go func() {
						select {
						case refc <- c.ref:
						case <-ctx.Done():
						}
					}()
				case '?':
					c.focus = help

				case 'v':
					if err := c.viewLog(ctx); err != nil {
						return err
					}
				default:
					c.table.Process(ev)
				}
			} else if ev.Key() == tcell.KeyEnter {
				c.nextMatch(true)
			} else {
				c.table.Process(ev)
			}
		}
	}

	c.draw()
	return nil
}

func RunApplication(ctx context.Context, newScreen func() (tcell.Screen, error), repo string, ref string, conf Configuration) error {
	// FIXME Discard log until the status bar is implemented in order to hide the "Unsolicited response received on
	//  idle HTTP channel" from GitLab's HTTP client
	log.SetOutput(ioutil.Discard)

	tableConfig, err := conf.TableConfig(defaultTableColumns)
	if err != nil {
		return err
	}

	// Keep this before NewTUI since it may use stdin/stderr for password prompt
	cacheDB, err := conf.Providers.ToCache(ctx)
	if err != nil {
		return err
	}

	encoding.Register()
	defaultStyle := tcell.StyleDefault
	if conf.Style.Default != nil {
		transform, err := conf.Style.Default.Parse()
		if err != nil {
			return err
		}
		defaultStyle = transform(defaultStyle)
	}
	ui, err := tui.NewTUI(newScreen, defaultStyle)
	if err != nil {
		return err
	}
	defer func() {
		// If another goroutine panicked this wouldn't run so we'd be left with a garbled screen.
		// The alternative would be to defer a call to recover for every goroutine that we launch
		// in order to have them return an error in case of a panic.
		ui.Finish()
	}()

	controllerConf := ControllerConfiguration{
		TableConfiguration: tableConfig,
		GitStyle:           tableConfig.NodeStyle.(providers.StepStyle).GitStyle,
	}

	controller, err := NewController(&ui, controllerConf, ref, cacheDB)
	if err != nil {
		return err
	}

	return controller.Run(ctx, repo)
}
