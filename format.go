package glogrotate

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Default format is logFile.YYYYMMDD.index

func DefaultPathFormatter(logFile string, index int) string {
	result := logFile + "." + time.Now().Format("20060102")
	if index > 0 {
		result += "." + strconv.Itoa(index)
	}
	return result
}

func DefaultPathParser(logFile, path string) (logTime time.Time, logIndex int, err error) {
	remaining := strings.TrimPrefix(path, logFile)
	if len(remaining) == len(path) {
		err = errors.New(`path does not start with logFile`)
		return
	}
	if len(remaining) == 0 || remaining[0] != '.' {
		err = errors.New(`invalid path format`)
		return
	}
	remaining = remaining[1:]
	tm := remaining[:8]
	if logTime, err = time.Parse(`20060102`, tm); err != nil {
		err = errors.New(`invalid path format`)
		return
	}
	remaining = remaining[8:]
	if len(remaining) > 0 {
		if remaining[0] != '.' {
			err = errors.New(`invalid path format`)
			return
		}
		remaining = remaining[1:]
		logIndex, err = strconv.Atoi(remaining)
		if err != nil {
			err = errors.New(`invalid path format`)
			return
		}
	}
	return
}
