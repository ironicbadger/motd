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
	flagWidth  = 24
	flagHeight = 6
	flagGap    = 3
	logoWidth  = 10
	logoHeight = 5
)

func placeImage(out io.Writer, name string, width, height int) {
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
			fmt.Fprintf(out, "\x1b_Ga=T,f=100,q=2,C=1,c=%d,r=%d,m=%d;%s\x1b\\", width, height, more, payload[offset:end])
		} else {
			fmt.Fprintf(out, "\x1b_Gm=%d;%s\x1b\\", more, payload[offset:end])
		}
	}
}

func ansiRows(name string) []string {
	data, _ := images.ReadFile("flags/" + name + ".ansi")
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func artwork(out io.Writer, cfg config, system operatingSystem, color, graphics bool) ([]string, int) {
	if !color {
		return nil, 0
	}
	var name string
	width, height := flagWidth, flagHeight
	switch cfg.Artwork {
	case "flag", "both":
		name = cfg.Country
	case "os":
		if !cfg.HideOS {
			name, width, height = system.logo, logoWidth, logoHeight
		}
	}
	if name == "" {
		return nil, 0
	}
	if graphics {
		placeImage(out, name, width, height)
		rows := make([]string, height)
		for i := range rows {
			rows[i] = strings.Repeat(" ", width)
		}
		return rows, width
	}
	return ansiRows(name), width
}

var countryFlags = [...]struct{ code, name string }{
	{"ca", "Canada"},
	{"us", "United States"},
	{"gb", "United Kingdom"},
}

func validCountry(code string) bool {
	for _, flag := range countryFlags {
		if flag.code == code {
			return true
		}
	}
	return false
}

func showFlags(out io.Writer, color, graphics bool) {
	sep := strings.Repeat(" ", flagGap)
	var names, labels [len(countryFlags)]string
	for i, flag := range countryFlags {
		names[i] = flag.name
		labels[i] = "[" + strings.ToUpper(flag.code) + "]"
		if i < len(countryFlags)-1 {
			names[i] = fmt.Sprintf("%-*s", flagWidth, names[i])
			labels[i] = fmt.Sprintf("%-*s", flagWidth, labels[i])
		}
	}
	fmt.Fprintln(out, strings.Join(names[:], sep))
	if !color {
		fmt.Fprintln(out, strings.Join(labels[:], sep))
		return
	}
	if graphics {
		for i, flag := range countryFlags {
			if i > 0 {
				fmt.Fprintf(out, "\x1b[%dC", flagWidth+flagGap)
			}
			placeImage(out, flag.code, flagWidth, flagHeight)
		}
		fmt.Fprint(out, "\r"+strings.Repeat("\n", flagHeight))
		return
	}
	var columns [len(countryFlags)][]string
	for i, flag := range countryFlags {
		columns[i] = ansiRows(flag.code)
	}
	for row := range flagHeight {
		var line [len(countryFlags)]string
		for col, rows := range columns {
			line[col] = rows[row]
		}
		fmt.Fprintln(out, strings.Join(line[:], sep))
	}
}
