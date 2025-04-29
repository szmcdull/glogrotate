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
	PathParser    func(logFile, path string) (logTime time.Time, logIndex int, err error)
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

func GetLatestFile(logPath string, pathParser PathParser, pathFormatter PathFormatter) (latestFile string, logTime time.Time, index int, err error) {
	dir := filepath.Dir(logPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		err = fmt.Errorf(`failed to read dir %s: %w`, dir, err)
		return
	}

	index = -1
	base := filepath.Base(logPath)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		logTime2, logIndex2, err := pathParser(base, entry.Name())
		if err != nil {
			continue
		}
		if logTime2.After(logTime) || logTime2.Equal(logTime) && logIndex2 > index {
			logTime = logTime2
			index = logIndex2
			latestFile = entry.Name()
		}
	}

	if index == -1 {
		index = 0
		latestFile = pathFormatter(logPath, index)
	} else {
		latestFile = filepath.Join(dir, latestFile)
	}

	return
}

func checkRotate(checker RotateChecker, args *RotateArgs, rotator Rotator, limiter Limiter, cleaner Cleaner, new bool) (rotated bool, err error) {
	needRotate, err := checker(args)
	if err != nil {
		return
	}
	if !needRotate && new {
		err = rotator(args.RealName, args.Path)
		if err != nil {
			return
		}
		// if _, err = os.Stat(args.RealName); err != nil {
		// 	needRotate = true
		// } else if _, err = os.Stat(args.Path); err != nil { // todo: check if Path exists but is not a symlink to RealName
		// 	needRotate = true
		// }
	}
	if needRotate {
		args.RealName = args.PathFormatter(args.Path, args.Index)
		err = rotator(args.RealName, args.Path)
		if err != nil {
			needRotate = false
			return
		}
		// cleanup old files in a separate goroutine
		go clean(*args, limiter, cleaner)
		return true, nil
	} else if new {
		go clean(*args, limiter, cleaner)
	}
	return
}

func clean(args RotateArgs, limiter Limiter, cleaner Cleaner) {
	oldFiles, err := limiter(args)
	if err != nil {
		log(`limiter: `, err.Error()) // todo: provide an option to specify the logger?
		return
	}
	err = cleaner(args, oldFiles)
	if err != nil {
		log(`cleaner: `, err.Error())
		return
	}
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

func SafeCloseChan[T any](ch chan<- T) {
	defer func() {
		recover()
	}()
	close(ch)
}

func log(args ...any) {
	now := time.Now().String()
	s := fmt.Sprint(args...)
	println(now, s)
}
