package glogrotate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultOpener open realFile
func DefaultOpener(logFile, realFile string) (file *os.File, err error) {
	// stat, err := os.Stat(logFile)
	// if err != nil {
	// 	if !errors.Is(err, os.ErrNotExist) {
	// 		return nil, fmt.Errorf(`failed to stat '%s': %w`, logFile, err)
	// 	}
	// } else {
	// 	if stat.IsDir() {
	// 		return nil, fmt.Errorf(`%s is a directory`, logFile)
	// 	}
	// 	linkDest, err2 := os.Readlink(logFile)
	// 	if err2 != nil {
	// 		return nil, fmt.Errorf(`%s is not a symlink or permission denied`, logFile)
	// 	}
	// 	if linkDest == realFile || linkDest == filepath.Base(realFile) {
	// 		return os.OpenFile(realFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	// 	}
	// 	if err = os.Remove(logFile); err != nil {
	// 		return nil, fmt.Errorf(`failed to remove '%s': %w`, logFile, err)
	// 	}
	// }

	file, err = os.OpenFile(realFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf(`open '%s': %w`, realFile, err)
	}
	// if err = os.Symlink(filepath.Base(realFile), logFile); err != nil {
	// 	file.Close()
	// 	return nil, fmt.Errorf(`failed to create symlink '%s' -> '%s': %w`, logFile, realFile, err)
	// }

	return
}

// DefaultRotator create newFile and symlink it to logFile
func DefaultRotator(newFile, logFile string) error {
	// check if logFile is a symlink. if logFile is a symlink but not point to newFile, remove the link
	stat, err := os.Stat(logFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(`rotate: Stat '%s': %w`, logFile, err)
		}
	} else {
		if stat.IsDir() {
			return fmt.Errorf(`rotate: %s is a directory`, logFile)
		}
		linkDest, err2 := os.Readlink(logFile)
		if err2 != nil {
			return fmt.Errorf(`rotate: ReadLink %s: %w`, logFile, err2)
		}
		if linkDest == newFile || linkDest == filepath.Base(newFile) {
			return nil
		}
		if err = os.Remove(logFile); err != nil {
			return fmt.Errorf(`rotate: failed to remove '%s': %w`, logFile, err)
		}
	}

	// create newFile and symlink it to logFile
	file, err := os.OpenFile(newFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	file.Close()

	err = os.Symlink(filepath.Base(newFile), logFile)
	return err
}

func DefaultChecker(args *RotateArgs) (needRotate bool, err error) {
	return false, nil
}

func NewRotateDaily() RotateChecker {
	nextRotateTime := int64(0)
	return func(args *RotateArgs) (needRotate bool, err error) {
		if nextRotateTime == 0 {
			now := time.Now()
			nextRotateTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1).UnixMicro()
		}
		needRotate = args.FileTime.UnixMicro() >= nextRotateTime
		if needRotate {
			nextRotateTime += 24 * 60 * 60 * 1000000
			args.Index = 0
		}
		return
	}
}

func NewRotateMaxSize(maxSize int) RotateChecker {
	return func(args *RotateArgs) (needRotate bool, err error) {
		needRotate = args.FileSize >= int64(maxSize)
		if needRotate {
			args.Index += 1
		}
		return
	}
}

func NewRotateMulti(rotators ...RotateChecker) RotateChecker {
	return func(args *RotateArgs) (needRotate bool, err error) {
		for _, rotator := range rotators {
			needRotate, err = rotator(args)
			if needRotate || err != nil {
				return
			}
		}
		return
	}
}
