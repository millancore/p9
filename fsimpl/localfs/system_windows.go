package localfs

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugelgupf/p9/p9"
)

// settableAttrs is what SetAttr applies. NTFS has no POSIX modes or owners,
// so those are accepted and ignored, as is CTime, which cannot be set.
var settableAttrs = p9.SetAttrMask{
	Size: true, ATime: true, MTime: true, CTime: true,
	ATimeNotSystemTime: true, MTimeNotSystemTime: true,
	Permissions: true, UID: true, GID: true,
}

func umask(_ int) int {
	return 0
}

// localToQid derives the QID path from the file's path. Reading the NTFS file
// index needs the file opened, which fails for links whose target denies
// access (the legacy junctions in every profile) and takes the whole readdir
// down with it, and costs three calls per lookup. NTFS paths are compared
// case-insensitively, so the hash takes them lowered. A renamed file gets a
// new QID, as on file systems without stable inode numbers.
func localToQid(path string, _ os.FileInfo) (uint64, error) {
	h := fnv.New64a()
	h.Write([]byte(strings.ToLower(filepath.Clean(path))))
	return h.Sum64(), nil
}

// lock implements p9.File.Lock with advisory POSIX record locks kept in the
// server, since NTFS only has mandatory ones.
func (l *Local) lock(pid int, locktype p9.LockType, flags p9.LockFlags, start, length uint64, client string) (p9.LockStatus, error) {
	return l.posixLock(pid, locktype, start, length, client)
}
