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

	var err error
	switch args[0] {
	case "to-python":
		if len(args) < 2 {
			printUsage()
			os.Exit(2)
		}
		err = runToPython(args[1:])
	case "to-sb3", "reverse":
		if len(args) < 2 {
			printUsage()
			os.Exit(2)
		}
		var output string
		if len(args) >= 3 {
			output = args[2]
		}
		err = converter.Reverse(converter.ReverseOptions{
			ProjectPath: args[1],
			OutputSB3:   output,
			SpecmapData: specmapData,
		})
	case "verify":
		if len(args) < 2 {
			printUsage()
			os.Exit(2)
		}
		err = converter.VerifyRoundTripProject(args[1], specmapData)
		if err == nil {
			fmt.Println("✅ 往返校验通过：project.py、Scratch project.json 与资源文件一致")
		}
	case "sync":
		if len(args) < 2 {
			printUsage()
			os.Exit(2)
		}
		err = converter.SyncRoundTripProject(args[1], specmapData)
		if err == nil {
			fmt.Println("✅ 已从 .sb3topy/project.json 重新生成 project.py")
		}
	default:
		// Backward-compatible form: sb3topy game.sb3 [output-dir]
		err = runToPython(args)
	}

	if err != nil {
		fmt.Printf("\n❌ 操作失败: %v\n", err)
		os.Exit(1)
	}
}

func runToPython(args []string) error {
	sb3Path := args[0]
	var outputDir string
	if len(args) >= 2 {
		outputDir = args[1]
	}

	if _, err := os.Stat(sb3Path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("无法找到 SB3 文件: %q", sb3Path)
		}
		return fmt.Errorf("无法访问 SB3 文件 %q: %w", sb3Path, err)
	}

	opts := converter.ConvertOptions{
		SB3Path:     sb3Path,
		OutputDir:   outputDir,
		EngineFS:    engineFS,
		SpecmapData: specmapData,
	}
	return converter.Run(opts)
}

func printUsage() {
	appName := filepath.Base(os.Args[0])
	fmt.Printf(`sb3topy CLI - Scratch 3 (.sb3) ↔ Python 可逆调试工具（原生 Go 版）

用法:
  %[1]s <项目文件.sb3> [输出目录]             # 兼容旧用法，等价于 to-python
  %[1]s to-python <项目文件.sb3> [输出目录]
  %[1]s verify <Python 项目目录|project.py>
  %[1]s sync <Python 项目目录|project.py>
  %[1]s to-sb3 <Python 项目目录|project.py> [输出文件.sb3]

推荐的代理调试流程:
  1. %[1]s to-python game.sb3 ./work/game
  2. 让代理分析 work/game/project.py
  3. 将可逆修改同步到 work/game/.sb3topy/project.json
  4. %[1]s sync ./work/game
  5. %[1]s verify ./work/game
  6. %[1]s to-sb3 ./work/game repaired.sb3

注意:
  任意 Python 代码不能可靠地反编译成 Scratch 积木。
  to-sb3 只接受由本工具生成、且 project.py 与 .sb3topy/project.json
  能通过 verify 的可逆工作区，从而避免静默生成损坏或丢失修改的 SB3。
`, appName)
}
