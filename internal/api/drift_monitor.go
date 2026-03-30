package api

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"arb/pkg/utils"
)

// restartScheduled tracks whether a drift-triggered restart has been scheduled.
var restartScheduled atomic.Bool

// driftRestartOnce ensures we only schedule one restart per process lifetime.
var driftRestartOnce sync.Once

// isSupervised returns true when the process is managed by a service supervisor
// (systemd) that will restart it on exit. systemd sets INVOCATION_ID for every
// service unit invocation.
func isSupervised() bool {
	return os.Getenv("INVOCATION_ID") != ""
}

// RuntimeProvenance captures the runtime state needed to assess binary drift.
type RuntimeProvenance struct {
	PID              int    `json:"pid"`
	ProcessStartTime string `json:"processStartTime"`
	ExePath          string `json:"exePath"`
	BinaryModTime    string `json:"binaryModTime,omitempty"`
	BinaryExists     bool   `json:"binaryExists"`
	BinaryDeleted    bool   `json:"binaryDeleted"`
	DriftDetected    bool   `json:"driftDetected"`
	DriftReason      string `json:"driftReason,omitempty"`
	RestartSupported bool   `json:"restartSupported"`
	RestartScheduled bool   `json:"restartScheduled"`
}

// runtimeProvenance builds the current drift assessment from live state.
func runtimeProvenance() RuntimeProvenance {
	p := RuntimeProvenance{
		PID:              os.Getpid(),
		ProcessStartTime: processStartTime.Format(time.RFC3339),
		RestartSupported: isSupervised(),
		RestartScheduled: restartScheduled.Load(),
	}

	exe, err := os.Executable()
	if err != nil {
		p.ExePath = "unknown"
		p.DriftDetected = true
		p.DriftReason = "cannot resolve executable path"
		return p
	}
	p.ExePath = exe

	info, err := os.Stat(exe)
	if err != nil {
		// Binary deleted from disk — Linux keeps the old inode mapped.
		p.BinaryDeleted = true
		p.DriftDetected = true
		p.DriftReason = "binary deleted from disk (replaced)"
		return p
	}

	p.BinaryExists = true
	p.BinaryModTime = info.ModTime().Format(time.RFC3339)

	if info.ModTime().After(processStartTime) {
		p.DriftDetected = true
		p.DriftReason = fmt.Sprintf("binary updated at %s, process started at %s",
			info.ModTime().Format("2006-01-02 15:04:05"),
			processStartTime.Format("2006-01-02 15:04:05"))
	}

	return p
}

// startBinaryDriftMonitor checks every 60s whether the on-disk binary has been
// replaced since this process started. When drift is detected:
//   - Supervised mode (systemd): alerts and schedules a one-shot auto-exit so
//     the supervisor restarts onto the new binary.
//   - Manual mode: alerts only — operator must restart manually.
//
// This prevents silent runtime drift where a build replaces the binary but the
// old process keeps serving traffic from the deleted inode (ARB-81, ARB-87).
func (s *Server) startBinaryDriftMonitor() {
	log := utils.NewLogger("drift-monitor")

	exe, err := os.Executable()
	if err != nil {
		log.Warn("cannot resolve executable path, drift monitor disabled: %v", err)
		return
	}

	alerted := false
	supervised := isSupervised()

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			info, err := os.Stat(exe)
			if err != nil {
				// Binary deleted from disk — Linux keeps the old inode mapped.
				if !alerted {
					alerted = true
					msg := fmt.Sprintf("BINARY DRIFT: on-disk binary %s no longer exists (replaced or deleted) — running process is stale, restart required", exe)
					log.Error("%s", msg)
					s.BroadcastAlert(map[string]interface{}{
						"type":    "binary_drift",
						"message": msg,
						"since":   processStartTime.Format(time.RFC3339),
					})
					if supervised {
						s.scheduleDriftRestart(log)
					}
				}
				continue
			}

			if info.ModTime().After(processStartTime) && !alerted {
				alerted = true
				msg := fmt.Sprintf("BINARY DRIFT: on-disk binary updated at %s but process started at %s — restart required to pick up changes",
					info.ModTime().Format("2006-01-02 15:04:05"),
					processStartTime.Format("2006-01-02 15:04:05"))
				log.Error("%s", msg)
				s.BroadcastAlert(map[string]interface{}{
					"type":      "binary_drift",
					"message":   msg,
					"binaryMod": info.ModTime().Format(time.RFC3339),
					"procStart": processStartTime.Format(time.RFC3339),
				})
				if supervised {
					s.scheduleDriftRestart(log)
				}
			}
		}
	}()

	mode := "manual"
	if supervised {
		mode = "supervised (auto-restart enabled)"
	}
	log.Info("binary drift monitor started (exe=%s, started=%s, mode=%s)", exe, processStartTime.Format("15:04:05"), mode)
}

// scheduleDriftRestart schedules a one-shot process exit after a grace period,
// allowing the supervisor (systemd with Restart=on-failure) to restart onto the
// new binary. Only fires once per process lifetime.
func (s *Server) scheduleDriftRestart(log *utils.Logger) {
	driftRestartOnce.Do(func() {
		restartScheduled.Store(true)
		grace := 5 * time.Second
		log.Warn("supervised mode: scheduling restart in %s for binary drift remediation", grace)
		s.BroadcastAlert(map[string]interface{}{
			"type":    "binary_drift_restart",
			"message": fmt.Sprintf("Auto-restart scheduled in %s — supervisor will respawn with new binary", grace),
			"grace":   grace.Seconds(),
		})
		go func() {
			time.Sleep(grace)
			log.Warn("sending SIGTERM for binary drift remediation — graceful shutdown will run")
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				log.Error("cannot find own process for SIGTERM, falling back to os.Exit: %v", err)
				os.Exit(1)
			}
			if err := p.Signal(syscall.SIGTERM); err != nil {
				log.Error("cannot send SIGTERM to self, falling back to os.Exit: %v", err)
				os.Exit(1)
			}
		}()
	})
}
