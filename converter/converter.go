package converter

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ConvertOptions stores settings for conversion
type ConvertOptions struct {
	SB3Path     string
	OutputDir   string
	EngineFS    fs.FS
	SpecmapData []byte
}

// Run performs the native Go SB3 to Python conversion process
func Run(opts ConvertOptions) error {
	// 1. Resolve output directory
	sb3Path := opts.SB3Path
	outputDir := opts.OutputDir
	if outputDir == "" {
		base := filepath.Base(sb3Path)
		ext := filepath.Ext(base)
		outputDir = strings.TrimSuffix(base, ext)
	}

	absOutDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("cannot resolve output directory: %w", err)
	}

	fmt.Printf("Converting SB3 project: %s\n", sb3Path)
	fmt.Printf("Output directory: %s\n\n", absOutDir)

	assetsDir := filepath.Join(absOutDir, "assets")
	engineDir := filepath.Join(absOutDir, "engine")

	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("cannot create assets directory: %w", err)
	}
	if err := os.MkdirAll(engineDir, 0755); err != nil {
		return fmt.Errorf("cannot create engine directory: %w", err)
	}

	// 2. Unpack SB3 (Zip)
	fmt.Println("[1/3] Extracting project data and archive entries...")
	r, err := zip.OpenReader(sb3Path)
	if err != nil {
		return fmt.Errorf("cannot open SB3 archive: %w", err)
	}
	defer r.Close()

	var projectJsonData []byte
	assetCount := 0
	assetEntries := make([]string, 0, len(r.File))
	seenEntries := make(map[string]bool, len(r.File))

	for _, f := range r.File {
		cleanName, isDir, err := validateArchiveEntryName(f.Name)
		if err != nil {
			return err
		}
		if seenEntries[cleanName] {
			return fmt.Errorf("duplicate ZIP entry %q", f.Name)
		}
		seenEntries[cleanName] = true

		if cleanName == "project.json" {
			if isDir || f.FileInfo().IsDir() {
				return fmt.Errorf("project.json must be a regular ZIP entry")
			}
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("cannot open project.json: %w", err)
			}
			projectJsonData, err = io.ReadAll(rc)
			closeErr := rc.Close()
			if err != nil {
				return fmt.Errorf("cannot read project.json: %w", err)
			}
			if closeErr != nil {
				return fmt.Errorf("cannot close project.json ZIP entry: %w", closeErr)
			}
			continue
		}

		entryName := cleanName
		if isDir || f.FileInfo().IsDir() {
			entryName += "/"
		}
		assetEntries = append(assetEntries, entryName)
		if err := extractZipEntry(f, assetsDir); err != nil {
			return fmt.Errorf("cannot extract %q: %w", f.Name, err)
		}
		assetCount++
	}

	if len(projectJsonData) == 0 {
		return fmt.Errorf("SB3 archive does not contain project.json")
	}

	fmt.Printf("      Extracted %d non-project entries into assets/\n", assetCount)

	// 3. Extract Embedded Engine Files
	fmt.Println("[2/3] Installing the Python runtime into engine/...")
	if err := ExtractEmbeddedFS(opts.EngineFS, "engine", engineDir); err != nil {
		return fmt.Errorf("cannot extract embedded engine files: %w", err)
	}

	// 4. Compile project.json directly in Go!
	fmt.Println("[3/3] Compiling project.json into project.py...")
	sm, err := LoadSpecMap(opts.SpecmapData)
	if err != nil {
		return fmt.Errorf("cannot load SpecMap data: %w", err)
	}

	pyCode, err := TranspileProject(projectJsonData, sm)
	if err != nil {
		return fmt.Errorf("cannot compile project.json: %w", err)
	}

	outPyPath := filepath.Join(absOutDir, "project.py")
	if err := os.WriteFile(outPyPath, []byte(pyCode), 0644); err != nil {
		return fmt.Errorf("cannot write project.py: %w", err)
	}

	if err := writeRoundTripMetadataWithAssets(absOutDir, projectJsonData, []byte(pyCode), sb3Path, assetEntries); err != nil {
		return fmt.Errorf("cannot write round-trip metadata: %w", err)
	}

	fmt.Printf("\nConversion completed: %s\n", absOutDir)
	fmt.Printf("Round-trip metadata: %s/\n", roundTripDirName)
	fmt.Printf("Run the project: cd \"%s\" && python3 project.py\n", outputDir)
	return nil
}

// ExtractEmbeddedFS recursively copies files from an fs.FS.
func ExtractEmbeddedFS(fsys fs.FS, currentDir string, targetBaseDir string) error {
	entries, err := fs.ReadDir(fsys, currentDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		currentPath := currentDir + "/" + entry.Name()
		targetPath := filepath.Join(targetBaseDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			if err := ExtractEmbeddedFS(fsys, currentPath, targetPath); err != nil {
				return err
			}
		} else {
			data, err := fs.ReadFile(fsys, currentPath)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
