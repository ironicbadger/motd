package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Flag images come from Flagcdn. Fedora and Ubuntu artwork comes from Simple Icons.
// The ANSI assets were rendered offline with Chafa 1.18.3.
//
//go:embed flags/*.ansi flags/*.png
var images embed.FS

const (
	flagWidth  = 20
	flagHeight = 5
	flagGap    = 3
	logoWidth  = 10
)

func placeImage(out io.Writer, name string, width int) {
	png, _ := images.ReadFile("flags/" + name + ".png")
	payload := base64.StdEncoding.EncodeToString(png)
	// Kitty limits each graphics payload to 4096 base64 bytes. C=1 keeps the cursor in place.
	for offset := 0; offset < len(payload); offset += 4096 {
		end := min(offset+4096, len(payload))
		more := 0
		if end < len(payload) {
			more = 1
		}
		if offset == 0 {
			fmt.Fprintf(out, "\x1b_Ga=T,f=100,q=2,C=1,c=%d,r=%d,m=%d;%s\x1b\\", width, flagHeight, more, payload[offset:end])
		} else {
			fmt.Fprintf(out, "\x1b_Gm=%d;%s\x1b\\", more, payload[offset:end])
		}
	}
}

func ansiRows(name string) []string {
	data, _ := images.ReadFile("flags/" + name + ".ansi")
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func artwork(out io.Writer, cfg config, system operatingSystem, color, graphics bool) []string {
	if !color {
		return nil
	}
	var name string
	width := flagWidth
	switch cfg.Artwork {
	case "flag":
		name = cfg.Country
	case "os":
		if !cfg.HideOS {
			name, width = system.logo, logoWidth
		}
	}
	if name == "" {
		return nil
	}
	if graphics {
		placeImage(out, name, width)
		rows := make([]string, flagHeight)
		for i := range rows {
			rows[i] = strings.Repeat(" ", width)
		}
		return rows
	}
	return ansiRows(name)
}

func showFlags(out io.Writer, color, graphics bool) {
	fmt.Fprintf(out, "%-*s   %-*s   %s\n", flagWidth, "Canada", flagWidth, "United States", "United Kingdom")
	if !color {
		fmt.Fprintf(out, "%-*s   %-*s   %s\n", flagWidth, "[CA]", flagWidth, "[US]", "[GB]")
		return
	}
	if graphics {
		placeImage(out, "ca", flagWidth)
		fmt.Fprintf(out, "\x1b[%dC", flagWidth+flagGap)
		placeImage(out, "us", flagWidth)
		fmt.Fprintf(out, "\x1b[%dC", flagWidth+flagGap)
		placeImage(out, "gb", flagWidth)
		fmt.Fprint(out, "\r"+strings.Repeat("\n", flagHeight))
		return
	}
	us, uk := ansiRows("us"), ansiRows("gb")
	for i, ca := range ansiRows("ca") {
		fmt.Fprintln(out, ca+"   "+us[i]+"   "+uk[i])
	}
}
