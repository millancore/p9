package localfs

import (
	"os"
	"path/filepath"

	"github.com/hugelgupf/p9/p9"
	"golang.org/x/sys/windows"
)

// StatFS implements p9.File.StatFS with the volume holding the file.
func (l *Local) StatFS() (p9.FSStat, error) {
	// GetDiskFreeSpaceEx takes a directory; statfs(2) takes any path.
	path := l.path
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		path = filepath.Dir(path)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return p9.FSStat{}, err
	}
	var avail, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &free); err != nil {
		return p9.FSStat{}, err
	}
	const bsize = 4096
	return p9.FSStat{
		Type:            0x01021997, // V9FS_MAGIC
		BlockSize:       bsize,
		Blocks:          total / bsize,
		BlocksFree:      free / bsize,
		BlocksAvailable: avail / bsize,
		Files:           1 << 32, // NTFS has no inode count to report
		FilesFree:       1 << 31,
		NameLength:      255,
	}, nil
}
