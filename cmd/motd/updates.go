package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type aptUpdateStatus struct {
	Available *int      `json:"available"`
	IndexDate time.Time `json:"index_date,omitzero"`
}

type updateStatus struct {
	APT        *aptUpdateStatus `json:"apt,omitempty"`
	ReadAt     time.Time        `json:"read_at"`
	Staged     string           `json:"staged,omitempty"`
	AutoReboot bool             `json:"auto_reboot,omitempty"`
	LastCheck  time.Time        `json:"last_check,omitzero"`
	NextCheck  time.Time        `json:"next_check,omitzero"`
	CheckState string           `json:"check_state,omitempty"`
}

func readUpdates() *updateStatus {
	if runtime.GOOS != "linux" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	path, err := exec.LookPath("rpm-ostree")
	if err != nil {
		return readAPTUpdates(ctx)
	}

	var deployments struct {
		Deployments []struct {
			Booted    bool   `json:"booted"`
			Staged    bool   `json:"staged"`
			Version   string `json:"version"`
			Reference string `json:"container-image-reference"`
		} `json:"deployments"`
	}
	data, err := exec.CommandContext(ctx, path, "status", "--json").Output()
	if err != nil || json.Unmarshal(data, &deployments) != nil || len(deployments.Deployments) == 0 {
		return nil
	}

	status := &updateStatus{ReadAt: time.Now()}
	var bootedReference, stagedReference string
	for _, deployment := range deployments.Deployments {
		if deployment.Booted {
			bootedReference = deployment.Reference
		}
		if deployment.Staged && !deployment.Booted {
			stagedReference = deployment.Reference
			status.Staged = deployment.Version
			if status.Staged == "" {
				status.Staged = "Image"
			}
		}
	}
	var timers []struct {
		Unit string `json:"unit"`
		Last int64  `json:"last"`
		Next int64  `json:"next"`
	}
	const checkTimer = "rpm-ostreed-automatic.timer"
	const rebootTimer = "ucore-reboot-if-staged.timer"
	data, err = exec.CommandContext(ctx, "systemctl", "list-timers", "--all", "--output=json", "--no-pager", checkTimer, rebootTimer).Output()
	if err != nil || json.Unmarshal(data, &timers) != nil {
		return status
	}
	for _, timer := range timers {
		switch timer.Unit {
		case rebootTimer:
			status.AutoReboot = timer.Next > 0 && strings.HasPrefix(bootedReference, "ostree-image-signed:") && bootedReference == stagedReference
		case checkTimer:
			if timer.Next > 0 {
				status.NextCheck = time.UnixMicro(timer.Next)
			}
			if timer.Last > 0 {
				status.LastCheck = time.UnixMicro(timer.Last)
			}
		}
	}
	if !status.LastCheck.IsZero() {
		data, err := exec.CommandContext(ctx, "systemctl", "show", "-p", "Result", "-p", "ActiveState", "rpm-ostreed-automatic.service").Output()
		if err == nil {
			switch {
			case strings.Contains(string(data), "ActiveState=activating\n"):
				status.CheckState = "running"
			case strings.Contains(string(data), "Result=success\n"):
				status.CheckState = "success"
			default:
				status.CheckState = "failed"
			}
		}
	}
	return status
}

func readAPTUpdates(ctx context.Context) *updateStatus {
	path, err := exec.LookPath("apt")
	if err != nil {
		return nil
	}
	apt := &aptUpdateStatus{}
	status := &updateStatus{ReadAt: time.Now(), APT: apt}

	// APT can report zero candidates with no indexes at all. Require package metadata.
	indexes, _ := filepath.Glob("/var/lib/apt/lists/*_Packages*")
	for _, index := range indexes {
		if info, err := os.Stat(index); err == nil && info.Size() > 0 && info.ModTime().After(apt.IndexDate) {
			apt.IndexDate = info.ModTime()
		}
	}
	if apt.IndexDate.IsZero() {
		return status
	}
	cmd := exec.CommandContext(ctx, path, "list", "--upgradable")
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	if out, err := cmd.Output(); err == nil {
		count := strings.Count(string(out), " [upgradable from:")
		apt.Available = &count
	}
	return status
}

func (s aptUpdateStatus) display(now time.Time) string {
	if s.Available == nil {
		return "APT status unavailable"
	}
	message := "No updates"
	switch count := *s.Available; {
	case count == 1:
		message = "1 package update available"
	case count > 1:
		message = fmt.Sprintf("%d package updates available", count)
	}
	if age := now.Sub(s.IndexDate); age >= 48*time.Hour {
		message += fmt.Sprintf(" · APT indexes %dd old", int(age.Hours()/24))
	}
	return message
}

func (s *updateStatus) display(now time.Time) string {
	if s == nil || now.Sub(s.ReadAt) >= 2*time.Hour {
		return ""
	}
	if s.APT != nil {
		return s.APT.display(now)
	}
	if s.Staged != "" {
		if s.AutoReboot {
			return s.Staged + " staged  ·  Auto-reboot pending"
		}
		return s.Staged + " staged  ·  Reboot pending"
	}
	lines := []string{"None staged"}
	switch s.CheckState {
	case "running":
		lines = append(lines, "Check running")
	case "success":
		lines = append(lines, "Checked "+s.LastCheck.Local().Format("15:04 MST"))
	case "failed":
		lines = append(lines, "Last check failed")
	}
	if s.NextCheck.After(now) {
		lines = append(lines, "Next "+s.NextCheck.Local().Format("15:04 MST"))
	}
	return strings.Join(lines, " · ")
}
