package glogrotate

import (
	"errors"
	"os"
)

// func NilCleaner(RotateArgs, []string) error {
// 	return nil
// }

func NewCleanerRemove() Cleaner {
	return func(args RotateArgs, oldFiles []string) error {
		errs := make([]error, len(oldFiles))
		for i, oldFile := range oldFiles {
			errs[i] = os.Remove(oldFile)
		}
		return errors.Join(errs...)
	}
}
