package glogrotate

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

type (
	BufioWriter struct {
		bufSize       int
		rotateHandler RotateChecker

		l      sync.Mutex
		writer *bufio.Writer
		file   *os.File
		RotateArgs
	}
)

// NewBufio creates a new bufio writer with default buffer size - 4096 at the time of release
func NewBufio(options Options, handler RotateChecker) (*BufioWriter, error) {
	return NewBufioSize(options, 0, handler)
}

func NewBufioSize(options Options, bufSize int, rotateChecker RotateChecker) (*BufioWriter, error) {
	if options.PathFormatter == nil {
		options.PathFormatter = DefaultPathFormatter
	}
	if options.PathParser == nil {
		options.PathParser = DefaultPathParser
	}
	if options.Rotator == nil {
		options.Rotator = DefaultRotator
	}

	result := &BufioWriter{
		RotateArgs: RotateArgs{
			Options: options,
		},
		rotateHandler: rotateChecker,
		bufSize:       bufSize,
	}

	realFile, index, err := GetLatestFile(options.Path, options.PathParser, options.PathFormatter)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest log file of %s: %w", options.Path, err)
	}
	result.Index = index
	result.RealName = realFile

	_, err = checkRotate(rotateChecker, &result.RotateArgs, result.Rotator)
	if err != nil {
		return nil, fmt.Errorf("failed to check rotation: %w", err)
	}

	err = result.open()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (me *BufioWriter) open() error {
	me.close()
	err := EnsureLink(me.Options.Path, me.RotateArgs.RealName)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(me.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", me.Path, err)
	}
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to state %s: %w", me.Path, err)
	}
	me.RotateArgs.FileSize = stat.Size()
	me.RotateArgs.FileTime = stat.ModTime()

	// realPath, err := os.Readlink(me.Path)
	// if err != nil {
	// 	return fmt.Errorf("failed to readlink %s: %w", me.Path, err)
	// }
	// me.RotateArgs.Index, err = me.PathParser(realPath, me.Path)
	// if err != nil {
	// 	return fmt.Errorf("failed to parse path %s: %w", me.Path, err)
	// }

	me.file = file
	me.writer = bufio.NewWriterSize(file, me.bufSize)
	return nil
}

func (me *BufioWriter) checkRotate() error {
	rotated, err := checkRotate(me.rotateHandler, &me.RotateArgs, me.Rotator)
	if err != nil {
		return fmt.Errorf("failed to check rotate: %w", err)
	}
	if rotated {
		err = me.open()
		if err != nil {
			return fmt.Errorf("failed to re-open log file: %w", err)
		}
	}
	return nil
}

func (me *BufioWriter) Write(p []byte) (n int, err error) {
	me.l.Lock()
	defer me.l.Unlock()

	me.RotateArgs.AppendSize = len(p)
	err = me.checkRotate()
	if err != nil {
		return 0, err
	}

	n, err = me.writer.Write(p)
	if err == nil {
		me.RotateArgs.FileSize += int64(n)
		me.FileTime = time.Now()
	}
	return
}

func (me *BufioWriter) Flush() error {
	me.l.Lock()
	defer me.l.Unlock()

	return me.writer.Flush()
}

func (me *BufioWriter) close() {
	me.file.Close()
}

func (me *BufioWriter) Close() {
	me.l.Lock()
	defer me.l.Unlock()

	me.close()
}
