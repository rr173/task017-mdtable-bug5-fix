# task017-mdtable

Markdown 表格生成与格式化服务，仅使用标准库将结构化的表头与数据行渲染为符合 GitHub Flavored Markdown（GFM）规范的表格文本，处理单元格转义、按显示宽度（CJK 全角字符算 2）对齐与分隔行生成，不依赖任何第三方库、数据库或外部服务。

## 本地运行

```bash
go run . server --addr :8080
go run . --smoke-test
```

主要接口：

- `GET /healthz`：健康检查。
- `POST /format`：提交 `{"header":[...],"rows":[[...]],"aligns":[...]}`，返回 `{"table":"...","widths":[...]}`。`aligns` 可省略（全部默认对齐），元素取值 `default`/`left`/`center`/`right`。
- `POST /validate`：与 `/format` 相同输入，只校验不渲染，返回 `{"ok":bool,"error":"..."}`。
- `POST /width`：提交 `{"text":"..."}`，返回 `{"width":N}`。

边界约束要点：

- 单元格内的 `|` 转义为 `\|`、`\` 转义为 `\\`；含原始换行符的单元格被拒绝。
- 列宽按转义后内容的显示宽度取最大值，下限 3；CJK 全角字符算 2，控制字符算 0。
- 分隔行每列总宽度等于该列宽度：默认纯 `-`、左 `:` 首、右 `:` 末、居中首尾 `:`。
- 数据行单元格数多于表头被拒；少于表头用空单元格补齐；表头为空或 aligns 长度与表头不一致被拒。

## Docker

镜像使用国内 DaoCloud Go 1.26.3 Bookworm builder 和 Alpine 3.20 runtime；支持 `linux/amd64` 与 `linux/arm64` 双架构。
