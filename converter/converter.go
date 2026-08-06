package converter

import (
	"archive/zip"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ConvertOptions stores settings for conversion
type ConvertOptions struct {
	SB3Path     string
	OutputDir   string
	EngineFS    embed.FS
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
		return fmt.Errorf("无法获取输出路径: %w", err)
	}

	fmt.Printf("📦 正在转换 SB3 项目: %s\n", sb3Path)
	fmt.Printf("📁 输出目录: %s\n\n", absOutDir)

	assetsDir := filepath.Join(absOutDir, "assets")
	engineDir := filepath.Join(absOutDir, "engine")

	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("创建 assets 目录失败: %w", err)
	}
	if err := os.MkdirAll(engineDir, 0755); err != nil {
		return fmt.Errorf("创建 engine 目录失败: %w", err)
	}

	// 2. Unpack SB3 (Zip)
	fmt.Println("[1/3] 解压并提取资源文件...")
	r, err := zip.OpenReader(sb3Path)
	if err != nil {
		return fmt.Errorf("无法读取 SB3 文件: %w", err)
	}
	defer r.Close()

	var projectJsonData []byte
	assetCount := 0

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("解压文件 %s 失败: %w", f.Name, err)
		}

		if f.Name == "project.json" {
			projectJsonData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return fmt.Errorf("读取 project.json 失败: %w", err)
			}
			continue
		}

		// Save asset to assetsDir
		dstPath := filepath.Join(assetsDir, f.Name)
		dstFile, err := os.Create(dstPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建资源文件 %s 失败: %w", dstPath, err)
		}
		_, err = io.Copy(dstFile, rc)
		dstFile.Close()
		rc.Close()
		if err != nil {
			return fmt.Errorf("写入资源文件 %s 失败: %w", dstPath, err)
		}
		assetCount++
	}

	if len(projectJsonData) == 0 {
		return fmt.Errorf("SB3 文件中未包含 project.json")
	}

	fmt.Printf("      已解压 %d 个资源文件到 assets/\n", assetCount)

	// 3. Extract Embedded Engine Files
	fmt.Println("[2/3] 部署 Python 运行时引擎到 engine/...")
	if err := ExtractEmbeddedFS(opts.EngineFS, "engine", engineDir); err != nil {
		return fmt.Errorf("提取 engine 引擎文件失败: %w", err)
	}

	// 4. Compile project.json directly in Go!
	fmt.Println("[3/3] 原生 Go 编译 project.json 为 project.py...")
	sm, err := LoadSpecMap(opts.SpecmapData)
	if err != nil {
		return fmt.Errorf("加载 SpecMap 数据失败: %w", err)
	}

	pyCode, err := TranspileProject(projectJsonData, sm)
	if err != nil {
		return fmt.Errorf("编译 project.json 失败: %w", err)
	}

	outPyPath := filepath.Join(absOutDir, "project.py")
	if err := os.WriteFile(outPyPath, []byte(pyCode), 0644); err != nil {
		return fmt.Errorf("写入 project.py 失败: %w", err)
	}

	fmt.Printf("\n✅ 原生 Go 转换成功！项目已保存至目录: %s\n", absOutDir)
	fmt.Printf("🚀 运行项目: cd \"%s\" && python3 project.py\n", outputDir)
	return nil
}

// ExtractEmbeddedFS recursively copies embedded files from embed.FS
func ExtractEmbeddedFS(fs embed.FS, currentDir string, targetBaseDir string) error {
	entries, err := fs.ReadDir(currentDir)
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
			if err := ExtractEmbeddedFS(fs, currentPath, targetPath); err != nil {
				return err
			}
		} else {
			data, err := fs.ReadFile(currentPath)
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
