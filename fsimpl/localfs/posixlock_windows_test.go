package localfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hugelgupf/p9/p9"
)

func TestPOSIXRecordLocks(t *testing.T) {
	dir := t.TempDir()
	root, err := Attacher(dir).Attach()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "db"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	_, a, _ := root.Walk([]string{"db"})
	_, b, _ := root.Walk([]string{"db"})

	if st, err := a.Lock(1, p9.WriteLock, 0, 0, 100, "vm"); st != p9.LockStatusOK || err != nil {
		t.Fatalf("first lock: %v %v", st, err)
	}
	if st, _ := a.Lock(1, p9.WriteLock, 0, 0, 100, "vm"); st != p9.LockStatusOK {
		t.Fatalf("the owner relocking its range: %v", st)
	}
	if st, _ := b.Lock(2, p9.ReadLock, 0, 50, 10, "vm"); st != p9.LockStatusBlocked {
		t.Fatalf("conflicting lock: %v, want Blocked", st)
	}
	if st, _ := b.Lock(2, p9.ReadLock, 0, 200, 10, "vm"); st != p9.LockStatusOK {
		t.Fatalf("disjoint lock: %v", st)
	}
	if st, _ := b.Lock(1, p9.ReadLock, 0, 50, 10, "other-vm"); st != p9.LockStatusBlocked {
		t.Fatalf("same pid on another client: %v, want Blocked", st)
	}

	typ, start, length, pid, client, err := b.(p9.LockGetter).GetLock(2, p9.WriteLock, 0, 0, "vm")
	if err != nil || typ != p9.WriteLock || start != 0 || length != 100 || pid != 1 || client != "vm" {
		t.Fatalf("GetLock = %v %d %d %d %q %v, want the WriteLock 0+100 held by pid 1", typ, start, length, pid, client, err)
	}
	if typ, _, _, _, _, _ := b.(p9.LockGetter).GetLock(2, p9.ReadLock, 300, 10, "vm"); typ != p9.Unlock {
		t.Fatalf("GetLock on a free range = %v, want Unlock", typ)
	}

	// Unlocking the middle splits the range, freeing only that part.
	a.Lock(1, p9.Unlock, 0, 40, 20, "vm")
	if st, _ := b.Lock(2, p9.WriteLock, 0, 45, 5, "vm"); st != p9.LockStatusOK {
		t.Fatalf("lock in the unlocked hole: %v", st)
	}
	if st, _ := b.Lock(2, p9.WriteLock, 0, 30, 5, "vm"); st != p9.LockStatusBlocked {
		t.Fatalf("lock in what is still held: %v, want Blocked", st)
	}

	// Length 0 runs to the end of the file, as in fcntl.
	a.Lock(1, p9.Unlock, 0, 0, 0, "vm")
	if st, _ := a.Lock(1, p9.WriteLock, 0, 1000, 0, "vm"); st != p9.LockStatusOK {
		t.Fatalf("lock to EOF: %v", st)
	}
	if st, _ := b.Lock(2, p9.ReadLock, 0, 1<<40, 1, "vm"); st != p9.LockStatusBlocked {
		t.Fatalf("lock far past a to-EOF lock: %v, want Blocked", st)
	}
}
