package converter

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func validateArchiveEntryName(name string) (clean string, isDir bool, err error) {
	if name == "" || strings.ContainsRune(name, '\x00') {
		return "", false, fmt.Errorf("invalid empty ZIP entry name")
	}
	if strings.Contains(name, `\`) {
		return "", false, fmt.Errorf("unsafe ZIP entry %q: backslashes are not allowed", name)
	}
	isDir = strings.HasSuffix(name, "/")
	raw := strings.TrimSuffix(name, "/")
	if raw == "" || strings.HasPrefix(raw, "/") || looksLikeWindowsAbsolutePath(raw) {
		return "", false, fmt.Errorf("unsafe ZIP entry path %q", name)
	}
	clean = path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != raw {
		return "", false, fmt.Errorf("unsafe or non-canonical ZIP entry path %q", name)
	}
	return clean, isDir, nil
}

func looksLikeWindowsAbsolutePath(name string) bool {
	return len(name) >= 2 && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) && name[1] == ':'
}

func extractionPath(baseDir, entryName string) (string, error) {
	clean, _, err := validateArchiveEntryName(entryName)
	if err != nil {
		return "", err
	}
	destination := filepath.Join(baseDir, filepath.FromSlash(clean))
	rel, err := filepath.Rel(baseDir, destination)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe ZIP entry path %q", entryName)
	}
	return destination, nil
}

func extractZipEntry(file *zip.File, baseDir string) error {
	clean, isDir, err := validateArchiveEntryName(file.Name)
	if err != nil {
		return err
	}
	if file.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe ZIP entry %q: symbolic links are not allowed", file.Name)
	}
	destination, err := extractionPath(baseDir, clean)
	if err != nil {
		return err
	}
	if isDir || file.FileInfo().IsDir() {
		return makeSafeDirectory(baseDir, destination)
	}
	if err := makeSafeDirectory(baseDir, filepath.Dir(destination)); err != nil {
		return err
	}
	if info, statErr := os.Lstat(destination); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to overwrite non-regular asset %q", destination)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("cannot inspect asset destination %q: %w", destination, statErr)
	}

	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("cannot open ZIP entry %q: %w", file.Name, err)
	}
	defer rc.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot create extracted asset %q: %w", destination, err)
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("cannot extract ZIP entry %q: %w", file.Name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("cannot close extracted asset %q: %w", destination, closeErr)
	}
	return nil
}

func makeSafeDirectory(baseDir, directory string) error {
	rel, err := filepath.Rel(baseDir, directory)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe extraction directory %q", directory)
	}
	current := baseDir
	if info, statErr := os.Lstat(current); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("extraction root %q is not a safe directory", baseDir)
		}
	} else if os.IsNotExist(statErr) {
		if err := os.MkdirAll(current, 0755); err != nil {
			return fmt.Errorf("cannot create extraction root %q: %w", baseDir, err)
		}
	} else {
		return fmt.Errorf("cannot inspect extraction root %q: %w", baseDir, statErr)
	}
	if rel == "." {
		return nil
	}
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		switch {
		case statErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()):
			return fmt.Errorf("unsafe extraction path component %q", current)
		case statErr == nil:
			continue
		case os.IsNotExist(statErr):
			if err := os.Mkdir(current, 0755); err != nil {
				return fmt.Errorf("cannot create extraction directory %q: %w", current, err)
			}
		default:
			return fmt.Errorf("cannot inspect extraction directory %q: %w", current, statErr)
		}
	}
	return nil
}
