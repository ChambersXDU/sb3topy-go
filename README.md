# sb3topy-go

Scratch 3.0 (`.sb3`) 与 Python/Pygame 调试工作区之间的原生 Go 转换工具。

本项目逻辑重构自 [BirdLogics/sb3topy](https://github.com/BirdLogics/sb3topy)，并增加了面向代码代理的 **安全往返调试**：先把 Scratch 转成更容易分析的 Python，再通过保留的 Scratch 元数据校验并重新打包为 `.sb3`。

> 任意 Python 代码无法可靠地反编译成 Scratch 积木。本项目的 `to-sb3` 只保证由本工具生成、并通过 `verify` 的可逆工作区，避免静默丢失修改。

## 编译

```bash
go build -o sb3topy
```

## Scratch → Python

兼容旧用法：

```bash
sb3topy game.sb3 ./work/game
```

也可以显式使用：

```bash
sb3topy to-python game.sb3 ./work/game
```

输出目录中会包含：

- `project.py`：供人和代码代理阅读、调试；
- `assets/`：Scratch 资源；
- `engine/`：Python 运行时；
- `.sb3topy/project.json`：可逆的 Scratch 源数据；
- `.sb3topy/manifest.json`：往返格式与校验信息。

生成的 Python 会带有源映射注释，例如：

```python
# sb3topy:block move-1 motion_movesteps
self.move(10)
```

这样代理可以从 Python 逻辑直接定位到原 Scratch block ID。

## 安全修改与校验

推荐流程：

```bash
sb3topy to-python game.sb3 ./work/game
sb3topy verify ./work/game

# 让代理分析 project.py，并把可逆修改落到 .sb3topy/project.json

sb3topy sync ./work/game
sb3topy verify ./work/game
sb3topy to-sb3 ./work/game repaired.sb3
```

`sync` 会从 `.sb3topy/project.json` 重新生成规范的 `project.py`；`verify` 会确认两者完全一致（忽略换行符差异），同时检查被引用的造型和声音资源是否存在。

如果只修改 `project.py` 而没有等价的 Scratch 表示，`verify` 和 `to-sb3` 会拒绝继续，而不是生成一个看似成功、实际丢失修改的文件。

## 给代码代理使用的 Skill

仓库内提供：

```text
skills/scratch-sb3-roundtrip/SKILL.md
```

它规定了从 SB3 转换、Python 分析、block ID 定位、安全修改、`sync`、`verify`、逆向打包以及失败诊断的完整流程。

## 运行转换后的 Python

```bash
cd work/game
python3 project.py
```

## 测试

```bash
go test ./...
```

## 许可

[MIT License](LICENSE)
