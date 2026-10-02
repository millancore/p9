// Copyright 2018 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package localfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugelgupf/p9/fsimpl/test"
	"github.com/hugelgupf/p9/p9"
)

func TestLocalFS(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "localfs-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	test.TestFile(t, Attacher(tempDir))
	test.TestReadOnlyFS(t, Attacher(tempDir))
	test.TestReadWriteFS(t, Attacher(tempDir))
}

func TestAppendWrites(t *testing.T) {
	dir := t.TempDir()
	root, err := Attacher(dir).Attach()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "log"), []byte("first\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, f, err := root.Walk([]string{"log"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Open(p9.WriteOnly | p9.OpenFlags(os.O_APPEND)); err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, line := range []string{"second\n", "third\n"} {
		// The offset is ignored in append mode, as pwrite(2) ignores it on Linux.
		if _, err := f.WriteAt([]byte(line), 0); err != nil {
			t.Fatalf("WriteAt in append mode: %v", err)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "log")); string(got) != "first\nsecond\nthird\n" {
		t.Errorf("Open with O_APPEND: content = %q", got)
	}

	_, d, err := root.Walk(nil)
	if err != nil {
		t.Fatal(err)
	}
	c, _, _, err := d.Create("new", p9.WriteOnly|p9.OpenFlags(os.O_APPEND), 0644, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, line := range []string{"a\n", "b\n"} {
		if _, err := c.WriteAt([]byte(line), 0); err != nil {
			t.Fatalf("WriteAt in append mode after Create: %v", err)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "new")); string(got) != "a\nb\n" {
		t.Errorf("Create with O_APPEND: content = %q", got)
	}
}

func TestSetAttrTimesAndSize(t *testing.T) {
	dir := t.TempDir()
	root, err := Attacher(dir).Attach()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	_, f, _ := root.Walk([]string{"f"})

	// touch -d sets both times to a given value.
	if err := f.SetAttr(p9.SetAttrMask{ATime: true, MTime: true, ATimeNotSystemTime: true, MTimeNotSystemTime: true},
		p9.SetAttr{ATimeSeconds: 1000000000, MTimeSeconds: 1000000000}); err != nil {
		t.Fatalf("explicit times: %v", err)
	}
	if fi, _ := os.Stat(p); fi.ModTime().Unix() != 1000000000 {
		t.Errorf("mtime = %v, want 1000000000", fi.ModTime().Unix())
	}

	// Setting only atime leaves mtime alone.
	if err := f.SetAttr(p9.SetAttrMask{ATime: true}, p9.SetAttr{}); err != nil {
		t.Fatalf("atime only: %v", err)
	}
	if fi, _ := os.Stat(p); fi.ModTime().Unix() != 1000000000 {
		t.Errorf("mtime changed to %v by an atime-only SetAttr", fi.ModTime().Unix())
	}

	// truncate(2) sends size with ctime and mtime.
	if err := f.SetAttr(p9.SetAttrMask{Size: true, MTime: true, CTime: true}, p9.SetAttr{Size: 4}); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if fi, _ := os.Stat(p); fi.Size() != 4 || fi.ModTime().Unix() == 1000000000 {
		t.Errorf("after truncate: size %d, mtime %v; want 4 and now", fi.Size(), fi.ModTime().Unix())
	}
}
