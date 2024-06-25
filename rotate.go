package glogrotate

import (
	"time"
)

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
