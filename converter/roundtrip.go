package converter

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	roundTripDirName      = ".sb3topy"
	roundTripProjectName  = "project.json"
	roundTripManifestName = "manifest.json"
	roundTripFormat       = "sb3topy-roundtrip"
	roundTripVersion      = 2
)

// ReverseOptions stores settings for converting a generated Python workspace back to SB3.
type ReverseOptions struct {
	ProjectPath string
	OutputSB3   string
	SpecmapData []byte
}

type roundTripManifest struct {
	Format            string   `json:"format"`
	Version           int      `json:"version"`
	SourceSB3         string   `json:"sourceSb3,omitempty"`
	ProjectJSONSHA256 string   `json:"projectJsonSha256"`
	ProjectPySHA256   string   `json:"projectPySha256"`
	AssetEntries      []string `json:"assetEntries"`
}

func writeRoundTripMetadata(projectDir string, projectJSON []byte, projectPy []byte, sourceSB3 string) error {
	assetEntries, err := collectAssetArchiveEntries(projectDir)
	if err != nil {
		return err
	}
	return writeRoundTripMetadataWithAssets(projectDir, projectJSON, projectPy, sourceSB3, assetEntries)
}

func writeRoundTripMetadataWithAssets(projectDir string, projectJSON []byte, projectPy []byte, sourceSB3 string, assetEntries []string) error {
	metaDir := filepath.Join(projectDir, roundTripDirName)
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return fmt.Errorf("cannot create round-trip metadata directory: %w", err)
	}
	assetEntries, err := normalizeAssetEntries(assetEntries)
	if err != nil {
		return fmt.Errorf("invalid round-trip asset list: %w", err)
	}

	projectPath := filepath.Join(metaDir, roundTripProjectName)
	if err := os.WriteFile(projectPath, projectJSON, 0644); err != nil {
		return fmt.Errorf("cannot save canonical project.json: %w", err)
	}

	manifest := roundTripManifest{
		Format:            roundTripFormat,
		Version:           roundTripVersion,
		SourceSB3:         filepath.Base(sourceSB3),
		ProjectJSONSHA256: sha256Hex(projectJSON),
		ProjectPySHA256:   sha256Hex(normalizeLineEndings(projectPy)),
		AssetEntries:      assetEntries,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode round-trip manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(metaDir, roundTripManifestName), manifestBytes, 0644); err != nil {
		return fmt.Errorf("cannot write round-trip manifest: %w", err)
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resolveProjectDir(projectPath string) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("Python workspace path is required")
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return "", fmt.Errorf("cannot access workspace path %q: %w", projectPath, err)
	}

	dir := projectPath
	if !info.IsDir() {
		dir = filepath.Dir(projectPath)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace path: %w", err)
	}
	return abs, nil
}

func loadRoundTripProject(projectPath string) (projectDir string, projectJSON []byte, projectPy []byte, manifest roundTripManifest, err error) {
	projectDir, err = resolveProjectDir(projectPath)
	if err != nil {
		return
	}

	metaDir := filepath.Join(projectDir, roundTripDirName)
	manifestBytes, readErr := os.ReadFile(filepath.Join(metaDir, roundTripManifestName))
	if readErr != nil {
		err = fmt.Errorf("missing %s/%s; run sb3topy to-python to create a reversible workspace: %w", roundTripDirName, roundTripManifestName, readErr)
		return
	}
	if unmarshalErr := json.Unmarshal(manifestBytes, &manifest); unmarshalErr != nil {
		err = fmt.Errorf("invalid round-trip manifest: %w", unmarshalErr)
		return
	}
	if manifest.Format != roundTripFormat || (manifest.Version != 1 && manifest.Version != roundTripVersion) {
		err = fmt.Errorf("unsupported round-trip format: format=%q version=%d", manifest.Format, manifest.Version)
		return
	}
	if !validSHA256(manifest.ProjectJSONSHA256) || !validSHA256(manifest.ProjectPySHA256) {
		err = fmt.Errorf("invalid round-trip manifest: SHA-256 fields must contain 64 hexadecimal characters")
		return
	}
	if manifest.Version >= 2 && manifest.AssetEntries == nil {
		err = fmt.Errorf("invalid round-trip manifest: assetEntries is required")
		return
	}
	manifest.AssetEntries, err = normalizeAssetEntries(manifest.AssetEntries)
	if err != nil {
		err = fmt.Errorf("invalid round-trip manifest asset list: %w", err)
		return
	}

	projectJSON, err = os.ReadFile(filepath.Join(metaDir, roundTripProjectName))
	if err != nil {
		err = fmt.Errorf("cannot read canonical project.json: %w", err)
		return
	}
	projectPy, err = os.ReadFile(filepath.Join(projectDir, "project.py"))
	if err != nil {
		err = fmt.Errorf("cannot read project.py: %w", err)
		return
	}
	return
}

// VerifyRoundTripProject verifies that project.py is exactly the code generated from
// .sb3topy/project.json (line-ending differences are ignored) and that referenced assets exist.
func VerifyRoundTripProject(projectPath string, specmapData []byte) error {
	projectDir, projectJSON, projectPy, manifest, err := loadRoundTripProject(projectPath)
	if err != nil {
		return err
	}

	sm, err := LoadSpecMap(specmapData)
	if err != nil {
		return fmt.Errorf("cannot load SpecMap data: %w", err)
	}
	expected, err := TranspileProject(projectJSON, sm)
	if err != nil {
		return fmt.Errorf("cannot regenerate Python from canonical project.json: %w", err)
	}

	if !bytes.Equal(normalizeLineEndings(projectPy), normalizeLineEndings([]byte(expected))) {
		return fmt.Errorf("project.py does not match the current generator output for .sb3topy/project.json; this may result from Python-only edits or a converter upgrade. Preserve any Python-only edits and apply reversible changes to .sb3topy/project.json, then run sync and verify")
	}
	if sha256Hex(projectJSON) != manifest.ProjectJSONSHA256 {
		return fmt.Errorf("canonical .sb3topy/project.json changed without a completed sync; run sb3topy sync before verify")
	}
	if sha256Hex(normalizeLineEndings(projectPy)) != manifest.ProjectPySHA256 {
		return fmt.Errorf("project.py does not match the round-trip manifest; run sb3topy sync before verify")
	}

	if err := validateReferencedAssets(projectDir, projectJSON); err != nil {
		return err
	}
	if err := validateOriginalArchiveEntries(projectDir, manifest.AssetEntries); err != nil {
		return err
	}
	return nil
}

func normalizeLineEndings(data []byte) []byte {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return []byte(s)
}

// SyncRoundTripProject regenerates project.py from the canonical round-trip project.json.
// It is intentionally one-way: Scratch JSON is the source of truth for reversible edits.
func SyncRoundTripProject(projectPath string, specmapData []byte) error {
	projectDir, projectJSON, _, manifest, err := loadRoundTripProject(projectPath)
	if err != nil {
		return err
	}

	sm, err := LoadSpecMap(specmapData)
	if err != nil {
		return fmt.Errorf("cannot load SpecMap data: %w", err)
	}
	projectPy, err := TranspileProject(projectJSON, sm)
	if err != nil {
		return fmt.Errorf("cannot regenerate Python from canonical project.json: %w", err)
	}
	if err := validateReferencedAssets(projectDir, projectJSON); err != nil {
		return err
	}
	if err := validateOriginalArchiveEntries(projectDir, manifest.AssetEntries); err != nil {
		return err
	}
	if manifest.Version == 1 {
		manifest.AssetEntries, err = collectAssetArchiveEntries(projectDir)
		if err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.py"), []byte(projectPy), 0644); err != nil {
		return fmt.Errorf("cannot write project.py: %w", err)
	}
	if err := writeRoundTripMetadataWithAssets(projectDir, projectJSON, []byte(projectPy), manifest.SourceSB3, manifest.AssetEntries); err != nil {
		return err
	}
	return nil
}

// Reverse packages a verified reversible Python workspace back into a Scratch 3 .sb3 file.
func Reverse(opts ReverseOptions) error {
	if err := VerifyRoundTripProject(opts.ProjectPath, opts.SpecmapData); err != nil {
		return err
	}

	projectDir, projectJSON, _, manifest, err := loadRoundTripProject(opts.ProjectPath)
	if err != nil {
		return err
	}

	outputPath := opts.OutputSB3
	if outputPath == "" {
		outputPath = projectDir + ".sb3"
	}
	if filepath.Ext(outputPath) == "" {
		outputPath += ".sb3"
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("cannot resolve output SB3 path: %w", err)
	}
	assetsDir := filepath.Join(projectDir, "assets")
	if inside, pathErr := pathIsWithin(assetsDir, absOutput); pathErr != nil {
		return pathErr
	} else if inside {
		return fmt.Errorf("output SB3 must not be placed inside the workspace assets directory")
	}
	var assetEntries []string
	if manifest.Version == 1 {
		assetEntries, err = collectAssetArchiveEntries(projectDir)
	} else {
		assetEntries, err = archiveEntriesForReverse(projectJSON, manifest.AssetEntries)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absOutput), 0755); err != nil {
		return fmt.Errorf("cannot create SB3 output directory: %w", err)
	}

	if info, statErr := os.Lstat(absOutput); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symbolic-link output %q", absOutput)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("cannot inspect output path %q: %w", absOutput, statErr)
	}
	out, err := os.CreateTemp(filepath.Dir(absOutput), ".sb3topy-output-*.sb3")
	if err != nil {
		return fmt.Errorf("cannot create temporary SB3 file: %w", err)
	}
	temporaryOutput := out.Name()
	if err := out.Chmod(0644); err != nil {
		_ = out.Close()
		_ = os.Remove(temporaryOutput)
		return fmt.Errorf("cannot set temporary SB3 permissions: %w", err)
	}
	success := false
	defer func() {
		_ = out.Close()
		if !success {
			_ = os.Remove(temporaryOutput)
		}
	}()

	zw := zip.NewWriter(out)
	if err := writeZipBytes(zw, "project.json", projectJSON); err != nil {
		_ = zw.Close()
		return err
	}

	for _, entry := range assetEntries {
		cleanName, isDir, err := validateArchiveEntryName(entry)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if isDir {
			if err := writeZipDirectory(zw, cleanName+"/"); err != nil {
				_ = zw.Close()
				return err
			}
			continue
		}
		assetPath := filepath.Join(assetsDir, filepath.FromSlash(cleanName))
		if err := writeZipFile(zw, cleanName, assetPath); err != nil {
			_ = zw.Close()
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot finalize SB3 archive: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("cannot close SB3 file: %w", err)
	}
	if err := os.Rename(temporaryOutput, absOutput); err != nil {
		return fmt.Errorf("cannot install completed SB3 output: %w", err)
	}
	success = true

	fmt.Printf("Reverse conversion completed: %s\n", absOutput)
	return nil
}

func writeZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("cannot create ZIP entry %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("cannot write ZIP entry %s: %w", name, err)
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name, sourcePath string) error {
	if _, isDir, err := validateArchiveEntryName(name); err != nil || isDir {
		if err != nil {
			return err
		}
		return fmt.Errorf("asset ZIP entry %q must be a file", name)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot inspect asset %s: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("asset %s must be a regular file", sourcePath)
	}
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot open asset %s: %w", sourcePath, err)
	}
	defer f.Close()

	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("cannot create asset ZIP entry %s: %w", name, err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("cannot write asset ZIP entry %s: %w", name, err)
	}
	return nil
}

func writeZipDirectory(zw *zip.Writer, name string) error {
	if _, isDir, err := validateArchiveEntryName(name); err != nil || !isDir {
		if err != nil {
			return err
		}
		return fmt.Errorf("ZIP directory entry %q must end with a slash", name)
	}
	_, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("cannot create ZIP directory entry %s: %w", name, err)
	}
	return nil
}

func validateReferencedAssets(projectDir string, projectJSON []byte) error {
	var project ProjectJSON
	if err := json.Unmarshal(projectJSON, &project); err != nil {
		return fmt.Errorf("invalid canonical project.json: %w", err)
	}

	assetsDir := filepath.Join(projectDir, "assets")
	for _, target := range project.Targets {
		for _, costume := range target.Costumes {
			name := referencedAssetName(costume.Md5Ext, costume.AssetID, costume.DataFormat)
			if name != "" {
				if err := validateAssetFile(assetsDir, name); err != nil {
					return fmt.Errorf("invalid or missing costume asset %q for target %q: %w", name, target.Name, err)
				}
			}
		}
		for _, sound := range target.Sounds {
			name := referencedAssetName(sound.Md5Ext, sound.AssetID, sound.DataFormat)
			if name != "" {
				if err := validateAssetFile(assetsDir, name); err != nil {
					return fmt.Errorf("invalid or missing sound asset %q for target %q: %w", name, target.Name, err)
				}
			}
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func normalizeAssetEntries(entries []string) ([]string, error) {
	normalized := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		clean, isDir, err := validateArchiveEntryName(entry)
		if err != nil {
			return nil, err
		}
		if clean == "project.json" {
			return nil, fmt.Errorf("asset list must not contain project.json")
		}
		if seen[clean] {
			return nil, fmt.Errorf("duplicate asset entry %q", entry)
		}
		seen[clean] = true
		if isDir {
			clean += "/"
		}
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func collectAssetArchiveEntries(projectDir string) ([]string, error) {
	assetsDir := filepath.Join(projectDir, "assets")
	rootInfo, err := os.Lstat(assetsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access assets directory: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, fmt.Errorf("workspace assets path must be a real directory")
	}

	var entries []string
	err = filepath.WalkDir(assetsDir, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == assetsDir {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed in assets: %s", current)
		}
		rel, err := filepath.Rel(assetsDir, current)
		if err != nil {
			return fmt.Errorf("cannot resolve asset path %q: %w", current, err)
		}
		archiveName := filepath.ToSlash(rel)
		if entry.IsDir() {
			archiveName += "/"
		} else {
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("cannot inspect asset %q: %w", current, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("asset %q is not a regular file", current)
			}
		}
		if _, _, err := validateArchiveEntryName(archiveName); err != nil {
			return err
		}
		entries = append(entries, archiveName)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot scan assets directory: %w", err)
	}
	return normalizeAssetEntries(entries)
}

func archiveEntriesForReverse(projectJSON []byte, originalEntries []string) ([]string, error) {
	entries := append([]string(nil), originalEntries...)
	var project ProjectJSON
	if err := json.Unmarshal(projectJSON, &project); err != nil {
		return nil, fmt.Errorf("invalid canonical project.json: %w", err)
	}
	for _, target := range project.Targets {
		for _, costume := range target.Costumes {
			if name := referencedAssetName(costume.Md5Ext, costume.AssetID, costume.DataFormat); name != "" {
				entries = append(entries, name)
			}
		}
		for _, sound := range target.Sounds {
			if name := referencedAssetName(sound.Md5Ext, sound.AssetID, sound.DataFormat); name != "" {
				entries = append(entries, name)
			}
		}
	}
	return normalizeAssetEntriesAllowDuplicates(entries)
}

func normalizeAssetEntriesAllowDuplicates(entries []string) ([]string, error) {
	unique := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		clean, isDir, err := validateArchiveEntryName(entry)
		if err != nil {
			return nil, err
		}
		if clean == "project.json" {
			return nil, fmt.Errorf("asset list must not contain project.json")
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		if isDir {
			clean += "/"
		}
		unique = append(unique, clean)
	}
	sort.Strings(unique)
	return unique, nil
}

func referencedAssetName(md5ext, assetID, dataFormat string) string {
	if md5ext != "" {
		return md5ext
	}
	if assetID != "" && dataFormat != "" {
		return assetID + "." + dataFormat
	}
	return ""
}

func validateOriginalArchiveEntries(projectDir string, entries []string) error {
	assetsDir := filepath.Join(projectDir, "assets")
	if err := validateAssetsRoot(assetsDir); err != nil {
		return err
	}
	for _, entry := range entries {
		clean, isDir, err := validateArchiveEntryName(entry)
		if err != nil {
			return err
		}
		assetPath := filepath.Join(assetsDir, filepath.FromSlash(clean))
		if err := validateExistingAssetPath(assetsDir, assetPath, isDir); err != nil {
			return fmt.Errorf("original SB3 entry %q is missing or unsafe: %w", entry, err)
		}
	}
	return nil
}

func validateAssetFile(assetsDir, name string) error {
	clean, isDir, err := validateArchiveEntryName(name)
	if err != nil {
		return err
	}
	if isDir {
		return fmt.Errorf("asset reference must name a file")
	}
	assetPath := filepath.Join(assetsDir, filepath.FromSlash(clean))
	return validateExistingAssetPath(assetsDir, assetPath, false)
}

func validateExistingAssetPath(baseDir, assetPath string, wantDir bool) error {
	if err := validateAssetsRoot(baseDir); err != nil {
		return err
	}
	rel, err := filepath.Rel(baseDir, assetPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("asset path escapes the assets directory")
	}
	current := baseDir
	components := strings.Split(rel, string(filepath.Separator))
	for index, component := range components {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic-link path component %q is not allowed", current)
		}
		last := index == len(components)-1
		if !last && !info.IsDir() {
			return fmt.Errorf("path component %q is not a directory", current)
		}
		if last && wantDir && !info.IsDir() {
			return fmt.Errorf("%q is not a directory", current)
		}
		if last && !wantDir && !info.Mode().IsRegular() {
			return fmt.Errorf("%q is not a regular file", current)
		}
	}
	return nil
}

func validateAssetsRoot(assetsDir string) error {
	info, err := os.Lstat(assetsDir)
	if err != nil {
		return fmt.Errorf("cannot access assets directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("workspace assets path must be a real directory")
	}
	return nil
}

func pathIsWithin(baseDir, candidate string) (bool, error) {
	rel, err := filepath.Rel(baseDir, candidate)
	if err != nil {
		return false, fmt.Errorf("cannot compare paths: %w", err)
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}
