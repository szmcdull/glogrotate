package glogrotate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func NilLimiter(args RotateArgs) (oldFiles []string, err error) {
	return
}

func NewLimiterDuration(duration time.Duration) Limiter {
	return func(args RotateArgs) (oldFiles []string, err error) {
		dir := filepath.Dir(args.Path)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read dir %s: %w", filepath.Dir(args.Path), err)
		}

		oldestTime := time.Now().Add(-duration)
		base := filepath.Base(args.Path)
		for _, entry := range entries {
			_, _, err2 := args.PathParser(base, entry.Name()) // check if the file is a log file
			if err2 != nil {
				continue
			}
			info, err2 := entry.Info()
			if err2 != nil {
				continue
			}
			if info.ModTime().Before(oldestTime) {
				oldFiles = append(oldFiles, filepath.Join(dir, entry.Name()))
			}
		}

		return
	}
}
