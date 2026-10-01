# sb3topy-go

连接 Scratch 使用者与 AI agent 的命令行工具：把 `.sb3` 转成便于 agent 分析的 Python 工作区，并把可逆的修复打包回 Scratch。

本项目逻辑重构自 [BirdLogics/sb3topy](https://github.com/BirdLogics/sb3topy)，并增加了面向代码代理的 **安全往返调试**：先把 Scratch 转成更容易分析的 Python，再通过保留的 Scratch 元数据校验并重新打包为 `.sb3`。

主要使用场景是学生在 Scratch 中制作作品，遇到问题后委托 agent 帮忙检查和修复。学生描述自己看到了什么、希望发生什么，agent 通过 CLI 定位具体角色和积木、完成修改，再由学生回到 Scratch 验证。学生不需要先学会命令行、Python 或 JSON。

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
self.move(tonum(10))
```

`hat` marker 也使用带引号的 `id=` / `opcode=` 属性。嵌套 SUBSTACK、reporter、shadow、custom procedure 以及当前不支持或不可达的 block 都会得到唯一 source association；`kind=reporter` 和 `kind=unmapped` 可帮助代理区分生成位置。这样代理可以从 Python 逻辑直接定位到原 Scratch block ID。

`.sb3topy/project.json` 保存输入 SB3 中 `project.json` 的原始字节，不会经由不完整的 Go struct 重新序列化。因此 `monitors`、`extensions`、`meta`、未来新增字段和 target/block 内未知字段都能保留。只有代理明确修改 canonical JSON 后，输出 SB3 中的 `project.json` 才会变化。

## 学生与 agent 协作

学生可以这样描述问题：

> 这是我的 Scratch 作品。点击绿旗后，角色碰到苹果应该加 1 分，但现在一直加分。请帮我找到原因，尽量保留其它功能，修好后给我能在 Scratch 中打开的文件，并告诉我怎么测试。

agent 应先结合学生提供的操作步骤分析作品，区分作品本身的逻辑问题与转换器或 Python 运行时的限制。最终向学生解释触发问题的原因、修改的角色和行为，以及在 Scratch 中验证修复的操作；block ID 和 JSON 修改细节用于技术记录，不必成为学生理解结果的前提。

## 只读检查与积木定位

转换前就可以查看 SB3 的角色、脚本入口、变量和列表，以及生成器明确报告的转换问题：

```bash
sb3topy inspect game.sb3
sb3topy inspect --json game.sb3
```

`--json` 在 stdout 输出完整 JSON，错误输出到 stderr，方便 agent 直接解析。报告带有 `schemaVersion`、角色索引、脚本 block ID 和事件字段（例如按键或广播名）、扩展列表、基础积木图检查和 Python 生成状态；`issues` 给出已检测到的未支持积木或输入，以及角色、block ID、opcode 和 `generatedPythonLine`。

需要查一个具体积木时，按学生说的角色名和源映射里的 ID 缩小范围：

```bash
sb3topy inspect --json --target "角色1" --block "BLOCK_ID" game.sb3
sb3topy inspect --json --block "BLOCK_ID" work/game
```

参数放在路径之前。选中的积木通过 `blocks[].raw` 返回原始字段，包括输入、菜单、`next`、`parent` 和未知字段。角色名使用精确匹配；同一个 block ID 出现在多个角色中时，报告会保留各自的角色索引。`generatedPythonLine` 是当前工具从 Scratch 源数据生成的 Python 中的源映射注释行号；工作区内的 Python 若有改动，行号不一定对应现有文件。

检查工作区时还会返回 `roundTrip` 校验结果。检查成功读取作品后，即使报告发现积木图错误或工作区不一致，也会以退出码 0 返回诊断，便于 agent 分析；无法读取文件、参数有误或找不到指定角色/积木时退出码为 1。`inspect` 不写文件、不更新 hash、不执行 `sync`。最终交付前仍需 `verify`，并在 Scratch 中测试：没有检测到转换问题不代表已经证明 Python 与 Scratch 的运行行为完全相同。

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

升级转换器后，旧工作区的 Python 可能与新生成器输出不同。先保留任何手动 Python 修改、把需要回到 Scratch 的改动落实到 canonical JSON，再使用新版本执行 `sync` 和 `verify`；`sync` 会覆盖 `project.py`。校验通过不代表 Python 与 Scratch 的运行行为完全一致，也不代表素材一定能解码，交付前仍需在 Scratch 中测试。

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
python3 -B -m unittest discover -s tests -v
go vet ./...
go build ./...
```

v1.2.1 的数值输入回归测试、38 个 SB3 样本验证结果及覆盖边界见 [验证记录](docs/validation-v1.2.1.md)。

## 许可

[MIT License](LICENSE)
