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
- `assets/`：原 SB3 中除根目录 `project.json` 外的所有 ZIP 条目（包括 Scratch 素材和其它文件）；
- `engine/`：Python 运行时；
- `.sb3topy/project.json`：可逆的 Scratch 源数据；
- `.sb3topy/manifest.json`：往返格式与校验信息。

生成的 Python 会带有源映射注释，例如：

```python
# sb3topy:block id="move-1" opcode="motion_movesteps" kind=stack shadow=false
self.move(10)
```

`hat` marker 也使用带引号的 `id=` / `opcode=` 属性。嵌套 SUBSTACK、reporter、shadow、custom procedure 以及当前不支持或不可达的 block 都会得到唯一 source association；`kind=reporter` 和 `kind=unmapped` 可帮助代理区分生成位置。这样代理可以从 Python 逻辑直接定位到原 Scratch block ID。

`.sb3topy/project.json` 保存输入 SB3 中 `project.json` 的原始字节，不会经由不完整的 Go struct 重新序列化。因此 `monitors`、`extensions`、`meta`、未来新增字段和 target/block 内未知字段都能保留。只有代理明确修改 canonical JSON 后，输出 SB3 中的 `project.json` 才会变化。

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

`sync` 会从 `.sb3topy/project.json` 重新生成规范的 `project.py`；`verify` 会确认两者完全一致（忽略换行符差异），并检查 manifest/hash、canonical JSON、原 SB3 ZIP 条目、被引用的造型和声音资源以及基础 block graph 完整性。`next`、`parent` 和 input/SUBSTACK 引用必须有效，明显循环会被拒绝。

如果只修改 `project.py` 而没有等价的 Scratch 表示，`verify` 和 `to-sb3` 会拒绝继续，而不是生成一个看似成功、实际丢失修改的文件。

SB3 解压会拒绝绝对路径、`..`、反斜杠路径、重复条目和符号链接，以防止 Zip Slip。逆向打包会恢复 manifest 记录的所有原始非 `project.json` 条目，并加入 canonical JSON 新引用的素材。

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
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

## 许可

[MIT License](LICENSE)
