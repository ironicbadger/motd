package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: motd show | motd refresh | motd flags (use -h after a command for options)")
	}
	command := args[0]
	switch command {
	case "show", "refresh", "flags":
	default:
		return fmt.Errorf("unknown command %q, use show, refresh, or flags", command)
	}

	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	cachePath := flags.String("cache", "", "path to the local cache")
	configPath := flags.String("config", "", "path to the display config")
	var host, osName, colorMode string
	var ansi bool
	if command != "refresh" {
		flags.StringVar(&host, "host", "", "override the displayed hostname")
		flags.StringVar(&osName, "os", "", "override the displayed OS label")
		flags.StringVar(&colorMode, "color", "auto", "ANSI colors (auto, always, never)")
		flags.BoolVar(&ansi, "ansi", false, "use terminal characters even when inline images are supported")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	var color, graphics bool
	var err error
	if command != "refresh" {
		color, graphics, err = terminalStyle(colorMode, ansi)
		if err != nil {
			return err
		}
	}
	if command == "flags" {
		showFlags(os.Stdout, color, graphics)
		return nil
	}
	configFile, cacheFile, err := resolvePaths(*configPath, *cachePath)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(configFile, *configPath == "")
	if err != nil {
		return err
	}
	if command == "refresh" {
		return refresh(cacheFile, cfg)
	}

	if host == "" {
		host = cfg.Hostname
	}
	if host == "" {
		host, _ = os.Hostname()
	}
	system := detectOS()
	if osName != "" {
		system.label = osName
	}
	show(os.Stdout, readCache(cacheFile), time.Now(), cfg, host, system, readStats(cfg), color, graphics)
	return nil
}

func terminalStyle(mode string, ansi bool) (color, graphics bool, err error) {
	switch mode {
	case "auto", "always", "never":
	default:
		return false, false, fmt.Errorf("unknown color mode %q", mode)
	}
	info, _ := os.Stdout.Stat()
	isTerminal := info != nil && info.Mode()&os.ModeCharDevice != 0
	term := os.Getenv("TERM")
	color = mode == "always" || (mode == "auto" && isTerminal && term != "dumb" && os.Getenv("NO_COLOR") == "")
	graphics = color && isTerminal && !ansi && (term == "xterm-ghostty" || term == "xterm-kitty")
	return color, graphics, nil
}
