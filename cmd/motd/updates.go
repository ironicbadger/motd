package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type updateStatus struct {
	ReadAt     time.Time `json:"read_at"`
	Staged     string    `json:"staged,omitempty"`
	AutoReboot bool      `json:"auto_reboot,omitempty"`
	LastCheck  time.Time `json:"last_check,omitzero"`
	NextCheck  time.Time `json:"next_check,omitzero"`
	CheckState string    `json:"check_state,omitempty"`
}

func readUpdates() *updateStatus {
	if runtime.GOOS != "linux" {
		return nil
	}
	path, err := exec.LookPath("rpm-ostree")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

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

func (s *updateStatus) display(now time.Time) string {
	if s == nil || now.Sub(s.ReadAt) >= 2*time.Hour {
		return ""
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
