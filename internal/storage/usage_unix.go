//go:build darwin || linux

package storage

import "syscall"

func Usage(path string) (total, used int64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return
	}
	total = int64(stat.Blocks) * int64(stat.Bsize)
	available := int64(stat.Bavail) * int64(stat.Bsize)
	used = total - available
	return
}
