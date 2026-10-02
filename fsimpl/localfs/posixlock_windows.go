package localfs

import (
	"sync"

	"github.com/hugelgupf/p9/p9"
)

// Advisory POSIX record locks, kept in the server since NTFS only has
// mandatory ones. Owners are the client's node name plus its pid, as F_SETLK
// identifies a process; ranges follow fcntl, where length 0 runs to the end.

type lockOwner struct {
	client string
	pid    int
}

type lockRange struct {
	owner      lockOwner
	write      bool
	start, end uint64 // end is exclusive, ^uint64(0) for "to EOF"
}

var (
	locksMu sync.Mutex
	locks   = map[string][]lockRange{}
)

func rangeEnd(start, length uint64) uint64 {
	if length == 0 || start+length < start {
		return ^uint64(0)
	}
	return start + length
}

func overlaps(a lockRange, start, end uint64) bool { return a.start < end && start < a.end }

// conflict returns a lock held by someone else that a request would collide with.
func conflict(path string, owner lockOwner, write bool, start, end uint64) (lockRange, bool) {
	for _, l := range locks[path] {
		if l.owner != owner && overlaps(l, start, end) && (write || l.write) {
			return l, true
		}
	}
	return lockRange{}, false
}

// release drops owner's coverage of [start,end), splitting ranges it cuts.
func release(path string, owner lockOwner, start, end uint64) {
	var kept []lockRange
	for _, l := range locks[path] {
		if l.owner != owner || !overlaps(l, start, end) {
			kept = append(kept, l)
			continue
		}
		if l.start < start {
			kept = append(kept, lockRange{owner: owner, write: l.write, start: l.start, end: start})
		}
		if end < l.end {
			kept = append(kept, lockRange{owner: owner, write: l.write, start: end, end: l.end})
		}
	}
	if len(kept) == 0 {
		delete(locks, path)
	} else {
		locks[path] = kept
	}
}

func (l *Local) posixLock(pid int, locktype p9.LockType, start, length uint64, client string) (p9.LockStatus, error) {
	owner := lockOwner{client: client, pid: pid}
	end := rangeEnd(start, length)
	locksMu.Lock()
	defer locksMu.Unlock()
	switch locktype {
	case p9.Unlock:
		release(l.path, owner, start, end)
		return p9.LockStatusOK, nil
	case p9.ReadLock, p9.WriteLock:
		write := locktype == p9.WriteLock
		if _, busy := conflict(l.path, owner, write, start, end); busy {
			// The Linux client retries a blocking request on Blocked.
			return p9.LockStatusBlocked, nil
		}
		release(l.path, owner, start, end)
		locks[l.path] = append(locks[l.path], lockRange{owner: owner, write: write, start: start, end: end})
		return p9.LockStatusOK, nil
	}
	return p9.LockStatusError, nil
}

// GetLock answers F_GETLK: the first lock that would block the request, or
// Unlock when it would be granted.
func (l *Local) GetLock(pid int, locktype p9.LockType, start, length uint64, client string) (p9.LockType, uint64, uint64, int, string, error) {
	owner := lockOwner{client: client, pid: pid}
	locksMu.Lock()
	defer locksMu.Unlock()
	c, busy := conflict(l.path, owner, locktype == p9.WriteLock, start, rangeEnd(start, length))
	if !busy {
		return p9.Unlock, start, length, pid, client, nil
	}
	t := p9.ReadLock
	if c.write {
		t = p9.WriteLock
	}
	n := c.end - c.start
	if c.end == ^uint64(0) {
		n = 0
	}
	return t, c.start, n, c.owner.pid, c.owner.client, nil
}
