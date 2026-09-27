package check

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Watch checks the project, then re-checks whenever one of its source files
// changes, until stop is closed (RGP1-076). Each run is a full check; the
// incremental program is RGP1-052.
func Watch(configPath string, interval time.Duration, stop <-chan struct{}, report func([]Report)) {
	root := filepath.Dir(configPath)
	last := snapshot(root)
	report(Run(configPath))
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			if now := snapshot(root); !sameSnapshot(last, now) {
				last = now
				report(Run(configPath))
			}
		}
	}
}

// snapshot records the size and modification time of every source file
// under root, skipping node_modules and hidden directories.
func snapshot(root string) map[string]string {
	files := map[string]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(p) {
		case ".rtsx", ".ts", ".tsx", ".mts", ".cts", ".json":
			if info, err := os.Stat(p); err == nil {
				files[p] = fmt.Sprintf("%d/%d", info.ModTime().UnixNano(), info.Size())
			}
		}
		return nil
	})
	return files
}

func sameSnapshot(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
