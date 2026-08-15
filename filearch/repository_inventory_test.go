package filearch

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestRepositoryInventorySkipsGitIgnoredUntrackedPaths(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", dir).Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	writeFile(t, dir, ".gitignore", ".env\n*.env\n.tmp/\n")
	writeFile(t, dir, ".env", "secret")
	writeFile(t, dir, ".tmp/cache", "temporary")
	writeFile(t, dir, "tracked.env", "must remain visible")
	writeFile(t, dir, "app.go", "package app")
	if err := exec.Command("git", "-C", dir, "add", "--force", "tracked.env").Run(); err != nil {
		t.Fatal(err)
	}

	inventory, err := newRepositoryInventory(dir)
	if err != nil {
		t.Fatal(err)
	}

	files, directories := inventory.relativeEntries(".", "recursive")
	if slices.Contains(files, ".env") {
		t.Fatalf("ignored file was inventoried: %v", files)
	}
	if strings.Contains(strings.Join(directories, ","), ".tmp") {
		t.Fatalf("ignored directory was inventoried: %v", directories)
	}
	if !inventory.hasRelativeFile(".", "app.go") {
		t.Fatal("non-ignored file was not inventoried")
	}
	if !inventory.hasRelativeFile(".", "tracked.env") {
		t.Fatal("tracked file matching .gitignore was not inventoried")
	}
}

func TestRepositoryInventoryIncludesRoot(t *testing.T) {
	inventory, err := newRepositoryInventory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	got := inventory.matchingDirectories(FileSet{Include: []string{"."}})
	if want := []string{"."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("matchingDirectories() = %v, want %v", got, want)
	}
}

func TestRepositoryInventoryQueriesRelativeFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "modules/one/metadata.go", "package one")
	writeFile(t, dir, "modules/one/features/a.go", "package features")
	writeFile(t, dir, "modules/ignored/metadata.go", "package ignored")

	inventory, err := newRepositoryInventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	dirs := inventory.matchingDirectories(FileSet{
		Include: []string{"modules/*"},
		Exclude: []string{"modules/ignored"},
	})
	if got := strings.Join(dirs, ","); got != "modules/one" {
		t.Fatalf("directories = %q", got)
	}
	if !inventory.hasRelativeFile("modules/one", "features/*.go") {
		t.Fatal("nested relative glob did not match")
	}
	if inventory.hasRelativeFile("modules/one", "missing.go") {
		t.Fatal("missing file matched")
	}
}

func TestRepositoryInventoryDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	external := t.TempDir()
	writeFile(t, external, "hidden.go", "package external")
	if err := os.MkdirAll(filepath.Join(dir, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dir, "modules", "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	inventory, err := newRepositoryInventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.hasRelativeFile("modules/linked", "hidden.go") {
		t.Fatal("inventory followed a directory symlink")
	}
}
