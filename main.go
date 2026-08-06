package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"sb3topy/converter"
)

//go:embed all:engine
var engineFS embed.FS

//go:embed specmap_data.json
var specmapData []byte

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "/?" {
		printUsage()
		os.Exit(0)
	}

	sb3Path := args[0]
	var outputDir string
	if len(args) >= 2 {
		outputDir = args[1]
	}

	if _, err := os.Stat(sb3Path); os.IsNotExist(err) {
		fmt.Printf("❌ 错误: 无法找到 SB3 文件: '%s'\n\n", sb3Path)
		printUsage()
		os.Exit(1)
	}

	opts := converter.ConvertOptions{
		SB3Path:     sb3Path,
		OutputDir:   outputDir,
		EngineFS:    engineFS,
		SpecmapData: specmapData,
	}

	if err := converter.Run(opts); err != nil {
		fmt.Printf("\n❌ 转换中出现错误: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	appName := filepath.Base(os.Args[0])
	fmt.Printf(`sb3topy CLI - Scratch 3 (.sb3) 到 Python 项目转换工具 (原生 Go 版)

用法:
  %s <项目文件.sb3> [输出路径]

参数说明:
  <项目文件.sb3>   指定需要转换的 Scratch 3 (.sb3) 文件路径 (必填)
  [输出路径]       指定生成的 Python 项目目标文件夹 (可选，默认同名)

示例:
  # 自动生成同名文件夹 game/
  %s game.sb3

  # 指定输出到 my_python_game/ 文件夹
  %s game.sb3 ./my_python_game
`, appName, appName, appName)
}
