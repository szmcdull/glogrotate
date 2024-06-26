package glogrotate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

/*
	Writer
		NewBufio				- create new bufio writer
			GetLatestFile		- get latest real log file and index
				PathParser		- parse real log file name to extract index
				PathFormatter	- combine logFile and index to create real log file name. Default format is logFile.YYYYMMDD.index
			open				- open symlink logFile, read file info (size, time, realName etc.)
				Opener			- create symlink if it does not exist or points to wrong file
		RotateChecker			- check if log file needs to be rotated. If it does, it must update index
		Rotator					- create real log file and symlink it to logFile. Rotator is coupled with Opener. If you customize one, you must also customize the other

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

	PathFormatter func(logFile string, index int) string
	PathParser    func(logFile, path string) (logIndex int, err error)
	Opener        func(logFile, realFile string) (*os.File, error)
	Rotator       func(newFile, logFile string) error
	RotateChecker func(args *RotateArgs) (needRotate bool, err error)
	Limiter       func(args RotateArgs) (oldFiles []string, err error)
	Cleaner       func(args RotateArgs, oldFiles []string) error

	Options struct {
		Path          string
		PathFormatter PathFormatter
		PathParser    PathParser
		Opener        Opener
		Rotator       Rotator
		RotateChecker RotateChecker
		Limiter       Limiter
		Cleaner       Cleaner
	}
)

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

func checkRotate(checker RotateChecker, args *RotateArgs, rotator Rotator, limiter Limiter, cleaner Cleaner) (rotated bool, err error) {
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
		// cleanup old files in a separate goroutine
		go func() {
			oldFiles, err := limiter(*args)
			if err != nil {
				println(`limiter: `, err.Error()) // todo: provide an option to specify the logger?
				return
			}
			err = cleaner(*args, oldFiles)
			if err != nil {
				println(`cleaner: `, err.Error())
				return
			}
		}()
		return true, nil
	}
	return
}

func DefaultOptions(options *Options) {
	if options.PathFormatter == nil {
		options.PathFormatter = DefaultPathFormatter
	}
	if options.PathParser == nil {
		options.PathParser = DefaultPathParser
	}
	if options.Opener == nil {
		options.Opener = DefaultOpener
	}
	if options.Rotator == nil {
		options.Rotator = DefaultRotator
	}
	if options.RotateChecker == nil {
		options.RotateChecker = DefaultChecker
	}
	if options.Limiter == nil {
		options.Limiter = NilLimiter
	}
	if options.Cleaner == nil {
		options.Cleaner = NewCleanerRemove() // would remove nothing if use default NilLimiter
	}

	absPath, err := filepath.Abs(options.Path)
	if err == nil {
		options.Path = absPath
	}
}
