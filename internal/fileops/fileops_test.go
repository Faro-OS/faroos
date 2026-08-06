package fileops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathTraversalIsBlocked(t *testing.T) {
	dir := t.TempDir()
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	// A file outside the root that traversal must not be able to reach.
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	if err := os.WriteFile(secret, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(secret)

	attempts := []string{
		"../secret.txt",
		"../../secret.txt",
		"../../../../../../etc/passwd",
		"foo/../../secret.txt",
		"/../secret.txt",
	}
	for _, a := range attempts {
		if _, err := root.ReadFile(a); err == nil {
			t.Errorf("ReadFile(%q) should have failed, escaped the root", a)
		}
	}
}

func TestWriteListReadDeleteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := root.WriteFile("notes/todo.txt", []byte("buy milk")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := root.List("notes")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "todo.txt" {
		t.Fatalf("expected [todo.txt], got %+v", entries)
	}

	data, err := root.ReadFile("notes/todo.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "buy milk" {
		t.Fatalf("expected 'buy milk', got %q", data)
	}

	if err := root.Delete("notes/todo.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := root.ReadFile("notes/todo.txt"); err == nil {
		t.Fatal("expected ReadFile to fail after Delete")
	}
}

func TestCannotDeleteRoot(t *testing.T) {
	dir := t.TempDir()
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Delete(""); err == nil {
		t.Fatal("expected deleting the root path to fail")
	}
	if err := root.Delete("/"); err == nil {
		t.Fatal("expected deleting the root path to fail")
	}
}
