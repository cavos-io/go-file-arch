package filearch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type repositoryInventory struct {
	root  string
	files map[string]struct{}
	dirs  []string
}

func newRepositoryInventory(root string) (*repositoryInventory, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve inventory root %q: %w", root, err)
	}
	inventory := &repositoryInventory{
		root:  absRoot,
		files: make(map[string]struct{}),
		dirs:  []string{"."},
	}
	err = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			inventory.dirs = append(inventory.dirs, rel)
			return nil
		}
		inventory.files[rel] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory %q: %w", absRoot, err)
	}
	sort.Strings(inventory.dirs)
	return inventory, nil
}

func (inventory *repositoryInventory) relativeEntries(baseDir, depth string) ([]string, []string) {
	baseDir = strings.Trim(filepath.ToSlash(baseDir), "/")
	if baseDir == "." {
		baseDir = ""
	}
	relative := func(path string) (string, bool) {
		if baseDir == "" {
			return path, true
		}
		prefix := baseDir + "/"
		if !strings.HasPrefix(path, prefix) {
			return "", false
		}
		return strings.TrimPrefix(path, prefix), true
	}
	include := func(path string) bool {
		return depth == "recursive" || !strings.Contains(path, "/")
	}

	var files []string
	for file := range inventory.files {
		if rel, ok := relative(file); ok && rel != "" && include(rel) {
			files = append(files, rel)
		}
	}
	var directories []string
	for _, directory := range inventory.dirs {
		if directory == "." || directory == baseDir {
			continue
		}
		if rel, ok := relative(directory); ok && rel != "" && include(rel) {
			directories = append(directories, rel)
		}
	}
	sort.Strings(files)
	sort.Strings(directories)
	return files, directories
}

func (inventory *repositoryInventory) matchingDirectories(files FileSet) []string {
	var matches []string
	for _, dir := range inventory.dirs {
		if MatchesAnyGlob(dir, files.Exclude) {
			continue
		}
		if MatchesAnyGlob(dir, files.Include) {
			matches = append(matches, dir)
		}
	}
	return matches
}

func (inventory *repositoryInventory) hasRelativeFile(baseDir, pattern string) bool {
	baseDir = strings.Trim(filepath.ToSlash(baseDir), "/")
	for file := range inventory.files {
		rel := file
		if baseDir != "" && baseDir != "." {
			prefix := baseDir + "/"
			if !strings.HasPrefix(file, prefix) {
				continue
			}
			rel = strings.TrimPrefix(file, prefix)
		}
		if matchGlob(rel, filepath.ToSlash(pattern)) {
			return true
		}
	}
	return false
}
