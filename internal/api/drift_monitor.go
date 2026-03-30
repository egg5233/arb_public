package api

import (
	"fmt"
	"os"
	"time"

	"arb/pkg/utils"
)

// startBinaryDriftMonitor checks every 60s whether the on-disk binary has been
// replaced since this process started. When drift is detected (binary mtime >
// process start), it logs a loud warning and broadcasts a dashboard alert so
// operators know the running process is stale.
//
// This prevents silent runtime drift where a build replaces the binary but the
// old process keeps serving traffic from the deleted inode (ARB-81).
func (s *Server) startBinaryDriftMonitor() {
	log := utils.NewLogger("drift-monitor")

	exe, err := os.Executable()
	if err != nil {
		log.Warn("cannot resolve executable path, drift monitor disabled: %v", err)
		return
	}

	alerted := false

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			info, err := os.Stat(exe)
			if err != nil {
				// Binary deleted from disk — Linux keeps the old inode mapped.
				// This is the classic "(deleted)" case.
				if !alerted {
					alerted = true
					msg := fmt.Sprintf("BINARY DRIFT: on-disk binary %s no longer exists (replaced or deleted) — running process is stale, restart required", exe)
					log.Error("%s", msg)
					s.BroadcastAlert(map[string]interface{}{
						"type":    "binary_drift",
						"message": msg,
						"since":   processStartTime.Format(time.RFC3339),
					})
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
			}
		}
	}()

	log.Info("binary drift monitor started (exe=%s, started=%s)", exe, processStartTime.Format("15:04:05"))
}
