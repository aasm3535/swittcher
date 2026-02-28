package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrHelp = errors.New("help requested")

type Command string

const (
	CommandTUI Command = "tui"
	CommandAdd Command = "add"
)

type Options struct {
	Command     Command
	CodexOnly   bool
	ClaudeOnly  bool
	GeminiOnly  bool
	ShowVersion bool
	ConfigDir   string
	AddApp      string
	AddName     string
}

func Parse(args []string) (Options, error) {
	opts := Options{Command: CommandTUI}

	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return Options{}, ErrHelp
		case arg == "-v" || arg == "--version":
			opts.ShowVersion = true
		case arg == "--codex":
			opts.CodexOnly = true
		case arg == "--claude":
			opts.ClaudeOnly = true
		case arg == "--gemini":
			opts.GeminiOnly = true
		case arg == "--config-dir":
			if i+1 >= len(args) {
				return Options{}, fmt.Errorf("--config-dir requires a value")
			}
			i++
			opts.ConfigDir = args[i]
		case strings.HasPrefix(arg, "--config-dir="):
			opts.ConfigDir = strings.TrimPrefix(arg, "--config-dir=")
		case strings.HasPrefix(arg, "-"):
			return Options{}, fmt.Errorf("unknown flag %q", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if (opts.CodexOnly && opts.ClaudeOnly) ||
		(opts.CodexOnly && opts.GeminiOnly) ||
		(opts.ClaudeOnly && opts.GeminiOnly) {
		return Options{}, fmt.Errorf("--codex, --claude and --gemini are mutually exclusive")
	}

	if len(positionals) == 0 {
		return opts, nil
	}

	switch positionals[0] {
	case "add":
		opts.Command = CommandAdd
		if len(positionals) < 2 {
			return Options{}, fmt.Errorf("missing app id for add command")
		}
		if len(positionals) > 3 {
			return Options{}, fmt.Errorf("too many arguments for add command")
		}
		opts.AddApp = positionals[1]
		if len(positionals) == 3 {
			opts.AddName = positionals[2]
		}
		return opts, nil
	default:
		return Options{}, fmt.Errorf("unknown command %q", positionals[0])
	}
}

func HelpText() string {
	return strings.TrimSpace(`
swittcher - switch CLI accounts in isolated profile homes

Usage:
  swittcher [--codex|--claude|--gemini] [--config-dir PATH]
  swittcher add <app> [profile-name] [--config-dir PATH]

Commands:
  add        Add a new account for an app and run login

Flags:
  --codex               Jump directly to codex account list
  --claude              Jump directly to claude account list
  --gemini              Jump directly to gemini account list
  -v, --version         Print version and exit
  --config-dir PATH     Override config directory (or use SWITTCHER_CONFIG_DIR)
  -h, --help            Show help
`) + "\n"
}

func PromptProfileName(in io.Reader, out io.Writer, appID string) (string, error) {
	if _, err := fmt.Fprintf(out, "Profile name for %s: ", appID); err != nil {
		return "", err
	}
	reader := bufio.NewReader(in)
	raw, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("profile name cannot be empty")
	}
	return name, nil
}
