package check

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Watch checks the project, then re-checks whenever one of its source files
// changes — or one is created or deleted: a segment file, an import's target
// — until stop is closed (RGP1-076). Each run is a full check.
func Watch(configPath string, interval time.Duration, stop <-chan struct{}, report func([]Report)) {
	configs := []string{configPath}
	var last map[string]string
	// check runs once. What the next snapshot is compared with is the one
	// taken before the run, so a file that changes during the run is seen;
	// the directories of projects the run found referenced join it as they
	// are after the run.
	check := func(before map[string]string) {
		watched := configs
		var reports []Report
		reports, configs, _ = run(configPath, nil)
		last = before
		for file, stamp := range snapshot(configs) {
			if !slices.ContainsFunc(watched, func(config string) bool {
				return strings.HasPrefix(file, filepath.Dir(config)+string(filepath.Separator))
			}) {
				last[file] = stamp
			}
		}
		report(reports)
	}
	check(snapshot(configs))
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			if now := snapshot(configs); !sameSnapshot(last, now) {
				check(now)
			}
		}
	}
}

// snapshot records the size and modification time of every source file
// under the directories of the tsconfig files — the project's and those of
// the projects it references — skipping node_modules and hidden directories.
func snapshot(configs []string) map[string]string {
	files := map[string]string{}
	for _, config := range configs {
		root := filepath.Dir(config)
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
			case ".rtsx", ".ts", ".tsx", ".mts", ".cts", ".json", ".jsx", ".js": // .jsx, .js: a segment's file (syntax.md, *Segment files*)
				if info, err := os.Stat(p); err == nil {
					files[p] = fmt.Sprintf("%d/%d", info.ModTime().UnixNano(), info.Size())
				}
			}
			return nil
		})
	}
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
