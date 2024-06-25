package glogrotate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

/*
	Writer
		NewBufio				- create new bufio writer
			GetLatestFile		- get latest real log file and index
				PathParser		- parse real log file name to extract index
				PathFormatter	- combine logFile and index to create real log file name. Default format is logFile.YYYYMMDD.index
			open				- open symlink logFile, read file info (size, time, realName etc.)
				EnsureLink		- create symlink if it does not exist or points to wrong file
		RotateChecker			- check if log file needs to be rotated. If it does, it must update index
		Rotator					- create real log file and symlink it to logFile

*/

type (
	RotateArgs struct {
		Options
		RealName   string    // real log file name
		FileSize   int64     // not accurate if file is shared between multiple processes
		FileTime   time.Time // last modification time
		Index      int       // rotate-index of the log file
		AppendSize int       // size of content to be appended when checking rotation
	}

	// RotateResult struct {
	// 	Rotated bool
	// }

	RotateChecker func(*RotateArgs) (needRotate bool, err error)
	PathFormatter func(logFile string, index int) string
	PathParser    func(logFile, path string) (logIndex int, err error)
	Rotator       func(newFile, logFile string) error

	Options struct {
		Path          string
		PathFormatter PathFormatter
		PathParser    PathParser
		Rotator       Rotator
	}
)

func DefaultPathFormatter(logFile string, index int) string {
	result := logFile + "." + time.Now().Format("20060102")
	if index > 0 {
		result += "." + strconv.Itoa(index)
	}
	return result
}

func DefaultPathParser(logFile, path string) (logIndex int, err error) {
	remaining := strings.TrimPrefix(path, logFile)
	if len(remaining) == len(path) {
		return 0, errors.New(`path does not start with logFile`)
	}
	if len(remaining) == 0 || remaining[0] != '.' {
		return 0, errors.New(`invalid path format`)
	}
	remaining = remaining[1:]
	tm := remaining[:8]
	if _, err = time.Parse(`20060102`, tm); err != nil {
		return 0, errors.New(`invalid path format`)
	}
	remaining = remaining[8:]
	if len(remaining) == 0 || remaining[0] != '.' {
		return 0, errors.New(`invalid path format`)
	}
	remaining = remaining[1:]
	logIndex, err = strconv.Atoi(remaining)
	if err != nil {
		return 0, errors.New(`invalid path format`)
	}
	return
}

// DefaultRotator create newFile and symlink it to logFile
func DefaultRotator(newFile, logFile string) error {
	os.Remove(logFile)
	file, err := os.OpenFile(newFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	file.Close()

	err = os.Symlink(newFile, logFile)
	return err
}

func GetLatestFile(logPath string, pathParser PathParser, pathFormatter PathFormatter) (latestFile string, index int, err error) {
	dir := filepath.Dir(logPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ``, 0, fmt.Errorf(`failed to read dir %s: %w`, dir, err)
	}

	index = -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		logIndex, err := pathParser(logPath, entry.Name())
		if err != nil {
			continue
		}
		if logIndex > index {
			index = logIndex
			latestFile = entry.Name()
		}
	}

	if index == -1 {
		index = 0
		latestFile = pathFormatter(logPath, index)
	}

	return
}

func EnsureLink(logFile, realFile string) error {
	stat, err := os.Stat(logFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(`failed to stat '%s': %w`, logFile, err)
		}
	} else {
		if stat.IsDir() {
			return fmt.Errorf(`%s is a directory`, logFile)
		}
		linkDest, err := os.Readlink(logFile)
		if err != nil {
			return fmt.Errorf(`%s is not a symlink or permission denied`, logFile)
		}
		if linkDest == realFile {
			return nil
		}
		if err = os.Remove(logFile); err != nil {
			return fmt.Errorf(`failed to remove '%s': %w`, logFile, err)
		}
	}

	f, err := os.OpenFile(realFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf(`failed to create '%s': %w`, realFile, err)
	}
	f.Close()
	if err = os.Symlink(realFile, logFile); err != nil {
		return fmt.Errorf(`failed to create symlink '%s' -> '%s': %w`, logFile, realFile, err)
	}
	return nil
}

func checkRotate(checker RotateChecker, args *RotateArgs, rotator Rotator) (rotated bool, err error) {
	needRotate, err := checker(args)
	if err != nil {
		return
	}
	if needRotate {
		args.RealName = args.PathFormatter(args.Path, args.Index)
		err = rotator(args.RealName, args.Path)
		if err != nil {
			needRotate = false
			return
		}
		return true, nil
	}
	return
}
