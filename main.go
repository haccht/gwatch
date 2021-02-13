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
	Version      = "1.0.2"
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
	cfg       config
	cache     string
	suspend   bool
	highlight int

	ui       *tview.Application
	title    *tview.TextView
	status   *tview.TextView
	datetime *tview.TextView
	footer   *tview.TextView
	content  *tview.TextView
	display  *tview.Flex
}

type config struct {
	ErrExit       bool    `short:"e" long:"errexit"  description:"Exit if command has a non-zero exit"`
	Interval      float64 `short:"n" long:"interval" description:"Time in seconds to wait between updates" default:"2.0"`
	NoTitle       bool    `short:"t" long:"no-title" description:"Turn off header"`
	Exec          bool    `short:"x" long:"exec"     description:"Pass command to exec instead of \"sh -c\""`
	HighlightMode string  `short:"m" long:"mode"     description:"Highlight mode" choice:"none" choice:"char" choice:"word" choice:"line" default:"none"`
	ColorStyle    string  `short:"s" long:"style"    description:"Interpret color and style sequences"`
	Version       func()  `short:"v" long:"version"  description:"Output version information and exit"`
}

func NewApp(cfg config) *App {
	a := &App{
		cfg:      cfg,
		ui:       tview.NewApplication(),
		title:    tview.NewTextView(),
		datetime: tview.NewTextView(),
		status:   tview.NewTextView(),
		content:  tview.NewTextView(),
		display:  tview.NewFlex(),
	}

	a.display.SetDirection(tview.FlexRow)
	if !a.cfg.NoTitle {
		header := tview.NewFlex()
		header.AddItem(a.title, 0, 1, false)
		header.AddItem(a.datetime, 24, 0, false)

		a.display.AddItem(header, 1, 0, false)
		a.display.AddItem(a.status, 1, 0, false)
	}
	a.display.AddItem(a.content, 0, 1, true)

	a.status.SetTextAlign(tview.AlignRight)
	a.status.SetDynamicColors(true)

	a.content.SetDynamicColors(true)
	a.content.SetChangedFunc(func() { a.ui.Draw() })
	a.content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'd':
			a.setHighlightMode((a.highlight + 1) % numHighlightMode)
		case 'p':
			a.setSuspendMode(!a.suspend)
		case '?':
			if a.footer == nil {
				a.showMessage("[j]Down [k]Up [h]Left [l]Right [g]Top [G]Bottom [d]Highlight [p]Pause [?]Help [q]Quit")
				a.ui.SetFocus(a.footer)
				a.footer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					a.hideMessage()
					a.ui.SetFocus(a.content)
					return event
				})
			}
		case 'q':
			a.ui.Stop()
			os.Exit(0)
		}
		return event
	})

	switch a.cfg.HighlightMode {
	case "char":
		a.setHighlightMode(HighlightModeChar)
	case "word":
		a.setHighlightMode(HighlightModeWord)
	case "line":
		a.setHighlightMode(HighlightModeLine)
	default:
		a.setHighlightMode(HighlightModeOff)
	}

	a.ui.SetRoot(a.display, true)
	return a
}

func (a *App) Start(args []string) {
	go a.tick(args)
	a.ui.Run()
}

func (a *App) showMessage(message string) {
	a.footer = tview.NewTextView()
	a.footer.SetText(message)
	a.display.AddItem(a.footer, 1, 0, false)
}

func (a *App) hideMessage() {
	a.display.RemoveItem(a.footer)
	a.footer = nil
}

func (a *App) setHighlightMode(mode int) {
	a.highlight = mode
	switch a.highlight {
	case HighlightModeOff:
		a.cfg.HighlightMode = "NONE"
	case HighlightModeChar:
		a.cfg.HighlightMode = "CHAR"
	case HighlightModeWord:
		a.cfg.HighlightMode = "WORD"
	case HighlightModeLine:
		a.cfg.HighlightMode = "LINE"
	}

	a.status.SetText(fmt.Sprintf("Highlight: [::u]%s[::-], press [d[] to change", a.cfg.HighlightMode))
}

func (a *App) setSuspendMode(mode bool) {
	a.suspend = mode
	if a.suspend {
		a.showMessage("Command execution is paused, press [p] to resume")
	} else {
		a.hideMessage()
		a.datetime.SetText(time.Now().Format(time.ANSIC))
	}
}

func (a *App) highlightContent(text string) string {
	if a.highlight == HighlightModeOff || a.cache == "" {
		a.cache = text
		return tview.Escape(text)
	}

	var split bufio.SplitFunc
	switch a.highlight {
	case HighlightModeChar:
		split = scanRunes
	case HighlightModeWord:
		split = scanWords
	case HighlightModeLine:
		split = scanLines
	}

	t1 := bufio.NewScanner(strings.NewReader(text))
	t1.Split(split)

	t2 := bufio.NewScanner(strings.NewReader(a.cache))
	t2.Split(split)

	var buf bytes.Buffer
	for t1.Scan() {
		token := t1.Text()
		if t2.Scan() && token == t2.Text() {
			fmt.Fprintf(&buf, "%s", tview.Escape(token))
		} else {
			fmt.Fprintf(&buf, "[%s]%s[-:-:-]", a.cfg.ColorStyle, tview.Escape(token))
		}
	}

	a.cache = text
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

	a.datetime.SetText(time.Now().Format(time.ANSIC))
	a.content.SetText(a.highlightContent(buf.String()))

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

	for {
		if errCode != 0 && a.cfg.ErrExit {
			break
		}

		<-t.C
		if !a.suspend {
			errCode = a.exec(cmdArgs)
		}
	}

	a.showMessage("Command exit with a non-zero status, press a key to exit")
	a.ui.SetFocus(a.footer)
	a.footer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		a.ui.Stop()
		os.Exit(errCode)
		return event
	})
}

func scanRunes(data []byte, atEOF bool) (int, []byte, error) {
	advance, token, err := bufio.ScanRunes(data, atEOF)
	if string(token) == "]" {
		return advance, []byte("[]"), err
	}

	return advance, token, err
}

func scanWords(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	r, width := utf8.DecodeRune(data)

	isDelim := unicode.IsSpace(r)
	scanNext := func(r rune) bool {
		return isDelim == unicode.IsSpace(r)
	}

	for j := width; j < len(data); j += width {
		r, width = utf8.DecodeRune(data[j:])
		if !scanNext(r) {
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
		return isDelim == (r == '\n')
	}

	for j := width; j < len(data); j += width {
		r, width = utf8.DecodeRune(data[j:])
		if !scanNext(r) {
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

	parser := flags.NewParser(&cfg, flags.Default|flags.PassAfterNonOption)
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

	if cfg.ColorStyle == "" {
		cfg.ColorStyle = DefaultStyle
	}

	app := NewApp(cfg)
	app.Start(args)
}
