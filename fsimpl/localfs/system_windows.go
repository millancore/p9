package localfs

import (
	"os"

	"github.com/hugelgupf/p9/p9"
	"golang.org/x/sys/windows"
)

func umask(_ int) int {
	return 0
}

func localToQid(path string, info os.FileInfo) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var (
		access     uint32 // none; we only need metadata
		sharemode  uint32
		createmode uint32 = windows.OPEN_EXISTING
		attribute  uint32 = windows.FILE_ATTRIBUTE_NORMAL
	)
	if info.IsDir() {
		attribute = windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	fd, err := windows.CreateFile(pathPtr, access, sharemode, nil, createmode, attribute, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(fd)

	fi := &windows.ByHandleFileInformation{}
	if err = windows.GetFileInformationByHandle(fd, fi); err != nil {
		return 0, err
	}

	x := uint64(fi.FileIndexHigh)<<32 | uint64(fi.FileIndexLow)
	return x, nil
}

// lock implements p9.File.Lock with advisory POSIX record locks kept in the
// server, since NTFS only has mandatory ones.
func (l *Local) lock(pid int, locktype p9.LockType, flags p9.LockFlags, start, length uint64, client string) (p9.LockStatus, error) {
	return l.posixLock(pid, locktype, start, length, client)
}
