# gwatch

`gwatch` is a tool to execute a program periodically, and show the output changes over time.
`gwatch` can highlight the differences between successive updates.

## Usage

```
Usage:
  gwatch [options] command

Application Options:
  -e, --errexit   exit if command has a non-zero exit
  -n, --interval= time in seconds to wait between updates (default: 2.0)
  -t, --no-title  turn off header
  -x, --exec      pass command to exec instead of "sh -c"
  -s, --style=    interpret color and style sequences
  -v, --version   output version information and exit

Help Options:
  -h, --help      Show this help message
```

## Keybinding

- <kbd>d</kbd>:  Change highlight mode between [NONE, CHAR, WORD, LINE].
- <kbd>p</kbd>:  Pause and resume command execution.
- <kbd>q</kbd>:  Quit
- <kbd>h</kbd>, <kbd>Left</kbd>:  Move left.
- <kbd>l</kbd>, <kbd>Right</kbd>:  Move right.
- <kbd>j</kbd>, <kbd>Down</kbd>:  Move down.
- <kbd>k</kbd>, <kbd>Up</kbd>:  Move up.
- <kbd>g</kbd>, <kbd>Home</kbd>:  Move to the top.
- <kbd>G</kbd>, <kbd>End</kbd>:  Move to the bottom.
- <kbd>Ctrl-f</kbd>, <kbd>PageDown</kbd>:  Move down by one page.
- <kbd>Ctrl-b</kbd>, <kbd>PageUp</kbd>:  Move up by one page.

## Examples

To watch the contents of a directory change, you could use

```sh
$ gwatch ls -l
```

To see the interface counters, you could use

```sh
$ gwatch -n 1 ip -s link show
```

You can apply your own style to highlight the output changes with the color tag

```sh
$ gwatch -s red ls -l
```

The full definition of a color tag is as follows:

```
<foreground>:<background>:<flags>
```

Color tags may contain not just the foreground color but also the background color and additional flags.
You can specify the following flags. Please refer to the [rivo/tview](https://pkg.go.dev/github.com/rivo/tview?tab=doc#hdr-Colors) for details.

```
l: blink
b: bold
d: dim
r: reverse (switch foreground and background color)
u: underline
```
