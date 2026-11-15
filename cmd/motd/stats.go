package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type systemStats struct {
	kernel  string
	cpuTemp string
	load    string
	uptime  string
}

func readStats(cfg config) systemStats {
	switch runtime.GOOS {
	case "linux":
		return readLinuxStats(cfg)
	case "darwin":
		return readDarwinStats(cfg)
	default:
		return systemStats{}
	}
}

func readLinuxStats(cfg config) systemStats {
	var stats systemStats
	if !cfg.HideKernel {
		stats.kernel = readText("/proc/sys/kernel/osrelease")
	}
	if !cfg.HideCPUTemp {
		stats.cpuTemp = linuxCPUTemperature("/sys")
	}
	if !cfg.HideLoad {
		if values := strings.Fields(readText("/proc/loadavg")); len(values) >= 3 {
			stats.load = strings.Join(values[:3], " ")
		}
	}
	if !cfg.HideUptime {
		if values := strings.Fields(readText("/proc/uptime")); len(values) > 0 {
			if seconds, err := strconv.ParseFloat(values[0], 64); err == nil && seconds >= 0 {
				stats.uptime = formatUptime(int64(seconds))
			}
		}
	}
	return stats
}

func readDarwinStats(cfg config) systemStats {
	var keys []string
	if !cfg.HideKernel {
		keys = append(keys, "kern.osrelease")
	}
	if !cfg.HideLoad {
		keys = append(keys, "vm.loadavg")
	}
	if !cfg.HideUptime {
		keys = append(keys, "kern.boottime")
	}
	var stats systemStats
	if len(keys) == 0 {
		return stats
	}
	data, _ := exec.Command("/usr/sbin/sysctl", keys...).Output()
	for _, line := range strings.Split(string(data), "\n") {
		key, value, _ := strings.Cut(line, ": ")
		switch key {
		case "kern.osrelease":
			stats.kernel = value
		case "vm.loadavg":
			var first, fifth, fifteenth float64
			if _, err := fmt.Sscanf(value, "{ %f %f %f }", &first, &fifth, &fifteenth); err == nil {
				stats.load = fmt.Sprintf("%.2f %.2f %.2f", first, fifth, fifteenth)
			}
		case "kern.boottime":
			var boot int64
			if _, err := fmt.Sscanf(value, "{ sec = %d,", &boot); err == nil && boot > 0 {
				if seconds := time.Now().Unix() - boot; seconds >= 0 {
					stats.uptime = formatUptime(seconds)
				}
			}
		}
	}
	return stats
}

// readText returns an empty value when a system reading is unavailable.
func readText(path string) string {
	data, _ := os.ReadFile(path)
	return strings.TrimSpace(string(data))
}

func formatUptime(seconds int64) string {
	clock := fmt.Sprintf("%02d:%02d:%02d", seconds/3600%24, seconds/60%60, seconds%60)
	if seconds >= 86400 {
		return fmt.Sprintf("%dd %s", seconds/86400, clock)
	}
	return clock
}

// linuxCPUTemperature reads CPU sensors exposed by Linux, excluding unrelated components such as GPUs and SSDs.
func linuxCPUTemperature(sysfs string) string {
	var hottest int64
	consider := func(path string) {
		if value, err := strconv.ParseInt(readText(path), 10, 64); err == nil && value > 0 && value <= 150000 {
			hottest = max(hottest, value)
		}
	}
	monitors, _ := filepath.Glob(filepath.Join(sysfs, "class/hwmon/hwmon*"))
	for _, monitor := range monitors {
		switch readText(filepath.Join(monitor, "name")) {
		case "coretemp", "k10temp", "zenpower", "cpu_thermal", "cpu-thermal":
			inputs, _ := filepath.Glob(filepath.Join(monitor, "temp*_input"))
			for _, input := range inputs {
				consider(input)
			}
		}
	}
	zones, _ := filepath.Glob(filepath.Join(sysfs, "class/thermal/thermal_zone*"))
	for _, zone := range zones {
		switch readText(filepath.Join(zone, "type")) {
		case "x86_pkg_temp", "cpu_thermal", "cpu-thermal", "soc_thermal", "soc-thermal":
			consider(filepath.Join(zone, "temp"))
		}
	}
	if hottest == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f°C", float64(hottest)/1000)
}
