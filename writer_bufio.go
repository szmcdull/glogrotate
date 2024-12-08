package glogrotate

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

type (
	// writer without lock, for async logger that writes from a dedicated go-proc
	BufioWriterNoLock struct {
		bufSize int
		exit    chan struct{}

		writer *bufio.Writer
		file   *os.File
		args   RotateArgs
	}

	BufioWriter struct {
		BufioWriterNoLock
		l sync.Mutex
	}
)

// NewBufio creates a new bufio writer with default buffer size - 4096 at the time of release
func NewBufio(options Options) (*BufioWriter, error) {
	return NewBufioSize(options, 0)
}

func NewBufioSize(options Options, bufSize int) (*BufioWriter, error) {
	DefaultOptions(&options)

	result := &BufioWriter{
		BufioWriterNoLock: BufioWriterNoLock{
			args: RotateArgs{
				Options: options,
			},
			bufSize: bufSize,
			exit:    make(chan struct{}),
		},
	}

	realFile, _, index, err := GetLatestFile(options.Path, options.PathParser, options.PathFormatter)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest log file of %s: %w", options.Path, err)
	}
	result.args.Index = index
	result.args.RealName = realFile

	err = result.checkRotate(true)
	// _, err = checkRotate(options.RotateChecker, &result.args, result.args.Rotator, result.args.Limiter, result.args.Cleaner)
	if err != nil {
		return nil, fmt.Errorf("failed to check rotation: %w", err)
	}

	// err = result.open()
	// if err != nil {
	// 	return nil, err
	// }

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if result.writer.Size() > 0 {
					result.l.Lock()
					err := result.writer.Flush()
					result.l.Unlock()
					if err != nil {
						log(err)
					}
				}
			case <-result.exit:
				return
			}
		}
	}()

	return result, nil
}

// NewBufio creates a new bufio writer with default buffer size - 4096 at the time of release
func NewBufioNoLock(options Options) (*BufioWriterNoLock, error) {
	return NewBufioNoLockSize(options, 0)
}

func NewBufioNoLockSize(options Options, bufSize int) (*BufioWriterNoLock, error) {
	DefaultOptions(&options)

	result := &BufioWriterNoLock{
		args: RotateArgs{
			Options: options,
		},
		bufSize: bufSize,
		exit:    make(chan struct{}),
	}

	realFile, _, index, err := GetLatestFile(options.Path, options.PathParser, options.PathFormatter)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest log file of %s: %w", options.Path, err)
	}
	result.args.Index = index
	result.args.RealName = realFile

	err = result.checkRotate(true)
	// _, err = checkRotate(options.RotateChecker, &result.args, result.args.Rotator, result.args.Limiter, result.args.Cleaner)
	if err != nil {
		return nil, fmt.Errorf("failed to check rotation: %w", err)
	}

	// err = result.open()
	// if err != nil {
	// 	return nil, err
	// }

	// go func() {
	// 	ticker := time.NewTicker(1 * time.Second)
	// 	defer ticker.Stop()

	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			if result.writer.Size() > 0 {
	// 				result.l.Lock()
	// 				err := result.writer.Flush()
	// 				result.l.Unlock()
	// 				if err != nil {
	// 					log(err)
	// 				}
	// 			}
	// 		case <-result.exit:
	// 			return
	// 		}
	// 	}
	// }()

	return result, nil
}

func (me *BufioWriterNoLock) open() error {
	me.close()
	file, err := me.args.Opener(me.args.Options.Path, me.args.RealName)
	if err != nil {
		return err
	}
	// file, err := os.OpenFile(me.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	// if err != nil {
	// 	return fmt.Errorf("failed to open file %s: %w", me.Path, err)
	// }
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to state %s: %w", me.args.Path, err)
	}
	me.args.FileSize = stat.Size()
	me.args.FileTime = stat.ModTime()

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

// checkRotate checks if the log file needs to be rotated. If it does, it will rotate the log file.
// new is true if the writer is being created.
func (me *BufioWriterNoLock) checkRotate(new bool) error {
	rotated, err := checkRotate(me.args.RotateChecker, &me.args, me.args.Rotator, me.args.Limiter, me.args.Cleaner, new)
	if err != nil {
		return fmt.Errorf("failed to check rotate: %w", err)
	}
	if rotated || new {
		err = me.open()
		if err != nil {
			return fmt.Errorf("failed to re-open log file: %w", err)
		}
	}
	return nil
}

func (me *BufioWriterNoLock) Write(p []byte) (n int, err error) {
	me.args.AppendSize = len(p)
	err = me.checkRotate(false)
	if err != nil {
		return 0, err
	}

	n, err = me.writer.Write(p)
	if err == nil {
		me.args.FileSize += int64(n)
		me.args.FileTime = time.Now()
	}
	return
}

func (me *BufioWriter) Write(p []byte) (n int, err error) {
	me.l.Lock()
	defer me.l.Unlock()
	return me.BufioWriterNoLock.Write(p)
}

func (me *BufioWriterNoLock) Flush() error {
	return me.writer.Flush()
}

func (me *BufioWriter) Flush() error {
	me.l.Lock()
	defer me.l.Unlock()
	return me.BufioWriterNoLock.Flush()
}

func (me *BufioWriterNoLock) close() {
	me.file.Close()
}

func (me *BufioWriterNoLock) Close() {
	me.close()
	SafeCloseChan(me.exit)
}

func (me *BufioWriter) Close() {
	me.l.Lock()
	defer me.l.Unlock()
	me.BufioWriterNoLock.Close()
}
