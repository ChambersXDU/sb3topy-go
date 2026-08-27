package converter

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	roundTripVersion      = 1
)

// ReverseOptions stores settings for converting a generated Python workspace back to SB3.
type ReverseOptions struct {
	ProjectPath string
	OutputSB3   string
	SpecmapData []byte
}

type roundTripManifest struct {
	Format            string `json:"format"`
	Version           int    `json:"version"`
	SourceSB3         string `json:"sourceSb3,omitempty"`
	ProjectJSONSHA256 string `json:"projectJsonSha256"`
	ProjectPySHA256   string `json:"projectPySha256"`
}

func writeRoundTripMetadata(projectDir string, projectJSON []byte, projectPy []byte, sourceSB3 string) error {
	metaDir := filepath.Join(projectDir, roundTripDirName)
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return fmt.Errorf("创建往返元数据目录失败: %w", err)
	}

	projectPath := filepath.Join(metaDir, roundTripProjectName)
	if err := os.WriteFile(projectPath, projectJSON, 0644); err != nil {
		return fmt.Errorf("保存原始 project.json 失败: %w", err)
	}

	manifest := roundTripManifest{
		Format:            roundTripFormat,
		Version:           roundTripVersion,
		SourceSB3:         filepath.Base(sourceSB3),
		ProjectJSONSHA256: sha256Hex(projectJSON),
		ProjectPySHA256:   sha256Hex(projectPy),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("生成往返 manifest 失败: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(metaDir, roundTripManifestName), manifestBytes, 0644); err != nil {
		return fmt.Errorf("写入往返 manifest 失败: %w", err)
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resolveProjectDir(projectPath string) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("未指定 Python 项目路径")
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return "", fmt.Errorf("无法访问项目路径 %q: %w", projectPath, err)
	}

	dir := projectPath
	if !info.IsDir() {
		dir = filepath.Dir(projectPath)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("无法解析项目路径: %w", err)
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
		err = fmt.Errorf("缺少 %s/%s；请先用新版 sb3topy 执行 to-python 生成可逆工作区: %w", roundTripDirName, roundTripManifestName, readErr)
		return
	}
	if unmarshalErr := json.Unmarshal(manifestBytes, &manifest); unmarshalErr != nil {
		err = fmt.Errorf("解析往返 manifest 失败: %w", unmarshalErr)
		return
	}
	if manifest.Format != roundTripFormat || manifest.Version != roundTripVersion {
		err = fmt.Errorf("不支持的往返格式: format=%q version=%d", manifest.Format, manifest.Version)
		return
	}

	projectJSON, err = os.ReadFile(filepath.Join(metaDir, roundTripProjectName))
	if err != nil {
		err = fmt.Errorf("读取可逆 project.json 失败: %w", err)
		return
	}
	projectPy, err = os.ReadFile(filepath.Join(projectDir, "project.py"))
	if err != nil {
		err = fmt.Errorf("读取 project.py 失败: %w", err)
		return
	}
	return
}

// VerifyRoundTripProject verifies that project.py is exactly the code generated from
// .sb3topy/project.json (line-ending differences are ignored) and that referenced assets exist.
func VerifyRoundTripProject(projectPath string, specmapData []byte) error {
	projectDir, projectJSON, projectPy, _, err := loadRoundTripProject(projectPath)
	if err != nil {
		return err
	}

	sm, err := LoadSpecMap(specmapData)
	if err != nil {
		return fmt.Errorf("加载 SpecMap 数据失败: %w", err)
	}
	expected, err := TranspileProject(projectJSON, sm)
	if err != nil {
		return fmt.Errorf("从可逆 project.json 重新生成 Python 失败: %w", err)
	}

	if !bytes.Equal(normalizeLineEndings(projectPy), normalizeLineEndings([]byte(expected))) {
		return fmt.Errorf("project.py 与 .sb3topy/project.json 不一致；Python-only 修改无法安全恢复为 Scratch。请把修改同步到 .sb3topy/project.json，然后执行 sync/verify")
	}

	if err := validateReferencedAssets(projectDir, projectJSON); err != nil {
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
		return fmt.Errorf("加载 SpecMap 数据失败: %w", err)
	}
	projectPy, err := TranspileProject(projectJSON, sm)
	if err != nil {
		return fmt.Errorf("从可逆 project.json 重新生成 Python 失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.py"), []byte(projectPy), 0644); err != nil {
		return fmt.Errorf("写入 project.py 失败: %w", err)
	}
	if err := writeRoundTripMetadata(projectDir, projectJSON, []byte(projectPy), manifest.SourceSB3); err != nil {
		return err
	}
	if err := validateReferencedAssets(projectDir, projectJSON); err != nil {
		return err
	}
	return nil
}

// Reverse packages a verified reversible Python workspace back into a Scratch 3 .sb3 file.
func Reverse(opts ReverseOptions) error {
	if err := VerifyRoundTripProject(opts.ProjectPath, opts.SpecmapData); err != nil {
		return err
	}

	projectDir, projectJSON, _, _, err := loadRoundTripProject(opts.ProjectPath)
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
		return fmt.Errorf("无法解析输出 SB3 路径: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOutput), 0755); err != nil {
		return fmt.Errorf("创建 SB3 输出目录失败: %w", err)
	}

	out, err := os.Create(absOutput)
	if err != nil {
		return fmt.Errorf("创建 SB3 文件失败: %w", err)
	}
	success := false
	defer func() {
		out.Close()
		if !success {
			_ = os.Remove(absOutput)
		}
	}()

	zw := zip.NewWriter(out)
	if err := writeZipBytes(zw, "project.json", projectJSON); err != nil {
		_ = zw.Close()
		return err
	}

	assetsDir := filepath.Join(projectDir, "assets")
	var assetPaths []string
	if err := filepath.Walk(assetsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		assetPaths = append(assetPaths, path)
		return nil
	}); err != nil {
		_ = zw.Close()
		return fmt.Errorf("扫描 assets 目录失败: %w", err)
	}
	sort.Strings(assetPaths)

	for _, assetPath := range assetPaths {
		rel, err := filepath.Rel(assetsDir, assetPath)
		if err != nil {
			_ = zw.Close()
			return fmt.Errorf("计算资源相对路径失败: %w", err)
		}
		if err := writeZipFile(zw, filepath.ToSlash(rel), assetPath); err != nil {
			_ = zw.Close()
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("完成 SB3 压缩失败: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("关闭 SB3 文件失败: %w", err)
	}
	success = true

	fmt.Printf("✅ 逆向转换成功: %s\n", absOutput)
	return nil
}

func writeZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("创建压缩条目 %s 失败: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("写入压缩条目 %s 失败: %w", name, err)
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name, sourcePath string) error {
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开资源 %s 失败: %w", sourcePath, err)
	}
	defer f.Close()

	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("创建压缩资源 %s 失败: %w", name, err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("写入压缩资源 %s 失败: %w", name, err)
	}
	return nil
}

func validateReferencedAssets(projectDir string, projectJSON []byte) error {
	var project ProjectJSON
	if err := json.Unmarshal(projectJSON, &project); err != nil {
		return fmt.Errorf("解析可逆 project.json 失败: %w", err)
	}

	assetsDir := filepath.Join(projectDir, "assets")
	for _, target := range project.Targets {
		for _, costume := range target.Costumes {
			name := costume.Md5Ext
			if name == "" {
				name = costume.AssetID + "." + costume.DataFormat
			}
			if name != "" {
				if _, err := os.Stat(filepath.Join(assetsDir, name)); err != nil {
					return fmt.Errorf("缺少造型资源 %q（角色 %q）: %w", name, target.Name, err)
				}
			}
		}
		for _, sound := range target.Sounds {
			name := sound.Md5Ext
			if name == "" {
				name = sound.AssetID + "." + sound.DataFormat
			}
			if name != "" {
				if _, err := os.Stat(filepath.Join(assetsDir, name)); err != nil {
					return fmt.Errorf("缺少声音资源 %q（角色 %q）: %w", name, target.Name, err)
				}
			}
		}
	}
	return nil
}
