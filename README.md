# sb3topy-go

Scratch 3.0 (`.sb3`) 到 Python / Pygame 项目的原生 Go 转换工具。

## 致谢 / Credits

本项目逻辑重构自原作者项目 [BirdLogics/sb3topy](https://github.com/BirdLogics/sb3topy.git)，感谢原作者的优秀工作。本项目使用 Go 语言重写，提供独立无依赖的单文件命令行体验。

## 编译

```bash
go build -o sb3topy
```

## 使用说明

```bash
sb3topy <项目文件.sb3> [输出路径]
```

示例：

```bash
# 默认生成同名文件夹 game/
sb3topy game.sb3

# 指定输出路径
sb3topy game.sb3 ./my_game
```

运行转换后的项目：

```bash
cd game
python3 project.py
```

## 许可

[MIT License](LICENSE)
