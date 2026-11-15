package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type operatingSystem struct {
	label string
	logo  string
}

func detectOS() operatingSystem {
	if runtime.GOOS == "linux" {
		for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
			if file, err := os.Open(path); err == nil {
				info := parseOSRelease(file)
				file.Close()
				return info
			}
		}
		return operatingSystem{label: "Linux"}
	}
	if runtime.GOOS == "darwin" {
		version, _ := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
		return operatingSystem{label: strings.TrimSpace("macOS " + string(version))}
	}
	return operatingSystem{label: runtime.GOOS}
}

func parseOSRelease(input io.Reader) operatingSystem {
	fields := make(map[string]string)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			fields[key] = strings.Trim(strings.TrimSpace(value), "'\"")
		}
	}
	label := fields["PRETTY_NAME"]
	if label == "" {
		label = strings.TrimSpace(fields["NAME"] + " " + fields["VERSION_ID"])
	}
	if label == "" {
		label = "Linux"
	}
	for _, id := range strings.Fields(fields["ID"] + " " + fields["ID_LIKE"]) {
		switch id {
		case "fedora", "ucore":
			return operatingSystem{label, "fedora"}
		case "ubuntu":
			return operatingSystem{label, "ubuntu"}
		}
	}
	return operatingSystem{label: label}
}
