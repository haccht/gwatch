package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell"
	"github.com/jessevdk/go-flags"
	"github.com/rivo/tview"
)

const (
	Version      = "1.0.0"
	DefaultStyle = "::r"
	MinInterval  = 0.1
)

const (
	HighlightModeOff = iota
	HighlightModeChar
	HighlightModeWord
	HighlightModeLine
	numHighlightMode
)

type App struct {
	cfg  config
	mode int

	ui       *tview.Application
	root     *tview.Flex
	title    *tview.TextView
	status   *tview.TextView
	datetime *tview.TextView
	content  *tview.TextView
}

type config struct {
	ErrExit  bool    `short:"e" long:"errexit"  description:"exit if command has a non-zero exit"`
	Interval float64 `short:"n" long:"interval" description:"time in seconds to wait between updates" default:"2.0"`
	NoTitle  bool    `short:"t" long:"no-title" description:"turn off header"`
	Exec     bool    `short:"x" long:"exec"     description:"pass command to exec instead of \"sh -c\""`
	Style    string  `short:"s" long:"style"    description:"interpret color and style sequences"`
	Version  func()  `short:"v" long:"version"  description:"output version information and exit"`
}

func NewApp(cfg config) *App {
	a := &App{
		cfg:      cfg,
		mode:     HighlightModeOff,
		ui:       tview.NewApplication(),
		root:     tview.NewFlex(),
		title:    tview.NewTextView(),
		status:   tview.NewTextView(),
		datetime: tview.NewTextView(),
		content:  tview.NewTextView(),
	}

	header := tview.NewFlex()
	header.AddItem(a.title, 0, 1, false)
	header.AddItem(a.datetime, 25, 0, false)

	a.root.SetDirection(tview.FlexRow)
	if !a.cfg.NoTitle {
		a.root.AddItem(header, 1, 0, false)
		a.root.AddItem(a.status, 1, 0, false)
	}
	a.root.AddItem(a.content, 0, 1, true)

	a.content.SetDynamicColors(true)
	a.content.SetChangedFunc(func() { a.ui.Draw() })
	a.content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'd':
			a.mode = (a.mode + 1) % numHighlightMode
			a.status.SetText(a.HighlightMode())
		case 'q':
			a.ui.Stop()
			os.Exit(0)
		}
		return event
	})

	a.datetime.SetDynamicColors(true)
	a.datetime.SetTextAlign(tview.AlignRight)

	a.status.SetDynamicColors(true)
	a.status.SetTextAlign(tview.AlignRight)
	a.status.SetText(a.HighlightMode())

	a.ui.SetRoot(a.root, true)
	return a
}

func (a *App) HighlightMode() string {
	switch a.mode {
	case HighlightModeChar:
		return "Highlight: [::u]CHAR[::-] - Press D to switch"
	case HighlightModeWord:
		return "Highlight: [::u]WORD[::-] - Press D to switch"
	case HighlightModeLine:
		return "Highlight: [::u]LINE[::-] - Press D to switch"
	}

	return "Highlight: [::u]OFF[::-]  - Press D to switch"
}

func (a *App) Start(args []string) {
	go a.tick(args)
	a.ui.Run()
}

func (a *App) highlight(s1, s2 string) string {
	if a.mode == HighlightModeOff || s2 == "" {
		return s1
	}

	var split bufio.SplitFunc
	switch a.mode {
	case HighlightModeChar:
		split = bufio.ScanRunes
	case HighlightModeWord:
		split = scanWords
	case HighlightModeLine:
		split = scanLines
	}

	t1 := bufio.NewScanner(strings.NewReader(s1))
	t1.Split(split)

	t2 := bufio.NewScanner(strings.NewReader(s2))
	t2.Split(split)

	var buf bytes.Buffer
	for t1.Scan() {
		token := t1.Text()
		if t2.Scan() && token == t2.Text() {
			fmt.Fprintf(&buf, "%s", token)
		} else {
			fmt.Fprintf(&buf, "[%s]%s[-:-:-]", a.cfg.Style, token)
		}
	}

	return buf.String()
}

func (a *App) exec(cmdArgs []string) int {
	var c *exec.Cmd
	if a.cfg.Exec {
		c = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	} else {
		c = exec.Command("sh", "-c", strings.Join(cmdArgs, " "))
	}

	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	err := c.Run()

	lastContent := a.content.GetText(true)
	currContent := buf.String()

	a.datetime.SetText(time.Now().Format(time.ANSIC))
	a.content.Clear()
	a.content.SetText(a.highlight(currContent, lastContent))

	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			if status, ok := e.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		}

		fmt.Fprintln(a.content, err.Error())
		return 1
	}

	return 0
}

func (a *App) tick(cmdArgs []string) {
	t := time.NewTicker(time.Duration(a.cfg.Interval*1000) * time.Millisecond)
	defer t.Stop()

	a.title.SetText(fmt.Sprintf("Every %.1fs: %s", a.cfg.Interval, strings.Join(cmdArgs, " ")))
	errCode := a.exec(cmdArgs)

TICK:
	for {
		if errCode != 0 && a.cfg.ErrExit {
			break TICK
		}

		<-t.C
		errCode = a.exec(cmdArgs)
	}

	footer := tview.NewTextView()
	footer.SetText("command exit with a non-zero status, press a key to exit")
	footer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		a.ui.Stop()
		os.Exit(errCode)
		return event
	})

	a.root.AddItem(footer, 1, 0, false)
	a.ui.SetFocus(footer)
}

func scanWords(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	r, width := utf8.DecodeRune(data)

	isDelim := unicode.IsSpace(r)
	scanNext := func(r rune) bool {
		return isDelim != unicode.IsSpace(r)
	}

	for j := width; j < len(data); j += width {
		r, width = utf8.DecodeRune(data[j:])
		if scanNext(r) {
			return j, data[:j], nil
		}
	}

	return 0, nil, nil
}

func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	r, width := utf8.DecodeRune(data)

	isDelim := (r == '\n')
	scanNext := func(r rune) bool {
		return isDelim != (r == '\n')
	}

	for j := width; j < len(data); j += width {
		r, width = utf8.DecodeRune(data[j:])
		if scanNext(r) {
			return j, data[:j], nil
		}
	}

	return 0, nil, nil
}

func main() {
	var cfg config
	cfg.Version = func() {
		fmt.Println(Version)
		os.Exit(0)
	}

	parser := flags.NewParser(&cfg, flags.Default|flags.IgnoreUnknown|flags.PassAfterNonOption)
	parser.Usage = "[options] command"

	args, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	} else if len(args) == 0 {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	if cfg.Interval < MinInterval {
		cfg.Interval = MinInterval
	}

	if cfg.Style == "" {
		cfg.Style = DefaultStyle
	}

	app := NewApp(cfg)
	app.Start(args)
}
