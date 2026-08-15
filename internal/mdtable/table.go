// Package mdtable 将结构化的表头与数据行渲染为符合 GitHub Flavored
// Markdown（GFM）规范的表格文本，处理单元格转义、显示宽度对齐与分隔行生成。
package mdtable

import (
	"errors"
	"fmt"
	"strings"
)

// Alignment 表示一列的对齐方式。
type Alignment int

const (
	AlignDefault Alignment = iota // 默认（左对齐，纯连字符分隔）
	AlignLeft                     // 左对齐
	AlignCenter                   // 居中对齐
	AlignRight                    // 右对齐
)

// ParseAlignment 将对齐方式字符串解析为 Alignment。
// 空字符串、""、"default" 表示默认对齐；接受大小写与前后空白。
func ParseAlignment(s string) (Alignment, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "default", "none":
		return AlignDefault, nil
	case "left", "l":
		return AlignLeft, nil
	case "center", "centre", "c", "middle":
		return AlignCenter, nil
	case "right", "r":
		return AlignRight, nil
	default:
		return 0, fmt.Errorf("非法对齐方式 %q", s)
	}
}

// String 返回对齐方式的可读名称，便于日志与错误信息。
func (a Alignment) String() string {
	switch a {
	case AlignLeft:
		return "left"
	case AlignCenter:
		return "center"
	case AlignRight:
		return "right"
	default:
		return "default"
	}
}

// Format 把表头与数据行渲染为 GFM 表格文本，返回表格文本与各列宽度。
// aligns 为 nil 时全部按默认对齐。返回的表格文本以单个换行结尾。
func Format(header []string, rows [][]string, aligns []Alignment) (string, []int, error) {
	if err := Validate(header, rows, aligns); err != nil {
		return "", nil, err
	}
	ncols := len(header)
	normAligns := make([]Alignment, ncols)
	if aligns != nil {
		copy(normAligns, aligns)
	}

	// 少于表头列数的数据行用空单元格补齐，多于表头的情形已被 Validate 拒绝。
	normRows := make([][]string, len(rows))
	for i, row := range rows {
		r := make([]string, ncols)
		copy(r, row)
		normRows[i] = r
	}

	// 列宽按转义后内容的显示宽度取最大值，下限为 3。
	widths := make([]int, ncols)
	for j := 0; j < ncols; j++ {
		w := displayWidth(escapeCell(header[j]))
		if w < 3 {
			w = 3
		}
		widths[j] = w
	}
	for _, row := range normRows {
		for j := 0; j < ncols; j++ {
			w := displayWidth(escapeCell(row[j]))
			if w > widths[j] {
				widths[j] = w
			}
		}
	}

	var b strings.Builder
	writeRow := func(padded []string) {
		b.WriteString("| ")
		for j := 0; j < ncols; j++ {
			if j > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(padded[j])
		}
		b.WriteString(" |\n")
	}

	// 表头行。
	paddedHeader := make([]string, ncols)
	for j := 0; j < ncols; j++ {
		paddedHeader[j] = padCell(escapeCell(header[j]), widths[j], normAligns[j])
	}
	writeRow(paddedHeader)

	// 分隔行：每列总宽度等于该列宽度，按对齐方式加冒号。
	b.WriteString("| ")
	for j := 0; j < ncols; j++ {
		if j > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(separatorCell(widths[j], normAligns[j]))
	}
	b.WriteString(" |\n")

	// 数据行。
	for _, row := range normRows {
		padded := make([]string, ncols)
		for j := 0; j < ncols; j++ {
			padded[j] = padCell(escapeCell(row[j]), widths[j], normAligns[j])
		}
		writeRow(padded)
	}

	return b.String(), widths, nil
}

// Validate 仅校验输入合法性，不渲染。返回的 error 描述第一处违规。
func Validate(header []string, rows [][]string, aligns []Alignment) error {
	if len(header) == 0 {
		return errors.New("表头不能为空")
	}
	if aligns != nil && len(aligns) != len(header) {
		return fmt.Errorf("对齐方式数组长度 %d 与表头列数 %d 不一致", len(aligns), len(header))
	}
	for i, a := range aligns {
		if a <= AlignDefault || a > AlignRight {
			return fmt.Errorf("第 %d 列对齐方式非法：%d", i, a)
		}
	}
	for j, h := range header {
		if containsNewline(h) {
			return fmt.Errorf("表头第 %d 列包含换行符", j)
		}
	}
	for i, row := range rows {
		if len(row) > len(header) {
			return fmt.Errorf("第 %d 行有 %d 个单元格，多于表头列数 %d", i, len(row), len(header))
		}
		for j, c := range row {
			if containsNewline(c) {
				return fmt.Errorf("第 %d 行第 %d 列包含换行符", i, j)
			}
		}
	}
	return nil
}

// DisplayWidth 返回 s 在等宽字体下的显示宽度：CJK 全角字符算 2，
// 控制字符算 0，其余可见字符算 1。该值与列宽计算所用的宽度一致。
func DisplayWidth(s string) int {
	return displayWidth(s)
}

// containsNewline 报告 s 是否包含原始换行符（\n 或 \r）。
func containsNewline(s string) bool {
	return strings.ContainsAny(s, "\n\r")
}

// escapeCell 转义单元格内容中的反斜杠与竖线：先反斜杠后竖线，
// 避免把转义引入的反斜杠二次转义。换行不在此处理，由 Validate 拒绝。
func escapeCell(s string) string {
	if !strings.ContainsAny(s, "\\|") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '|':
			b.WriteString(`\|`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// padCell 把已转义的内容按对齐方式填充到列宽。
func padCell(content string, width int, align Alignment) string {
	pad := width - displayWidth(content)
	if pad < 0 {
		pad = 0
	}
	switch align {
	case AlignRight:
		return strings.Repeat(" ", pad) + content
	case AlignCenter:
		left := pad / 2
		return strings.Repeat(" ", left) + content + strings.Repeat(" ", pad-left-1)
	default: // AlignLeft 与 AlignDefault 均左对齐
		return content + strings.Repeat(" ", pad)
	}
}

// separatorCell 生成总宽度等于 width 的分隔单元格。
// width 已保证 >= 3，因此左/右对齐至少 2 个连字符、居中至少 1 个连字符。
func separatorCell(width int, align Alignment) string {
	switch align {
	case AlignLeft:
		return ":" + strings.Repeat("-", width-1)
	case AlignRight:
		return ":" + strings.Repeat("-", width-1)
	case AlignCenter:
		return ":" + strings.Repeat("-", width-2) + ":"
	default:
		return strings.Repeat("-", width)
	}
}

// displayWidth 是 DisplayWidth 的内部实现，按 rune 求和。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// runeWidth 返回单个 rune 的显示宽度。
func runeWidth(r rune) int {
	if r == 0 || r < 0x10 || r == 0x7f {
		return 0 // 控制字符
	}
	if isWide(r) {
		return 2
	}
	return 1
}

// isWide 近似判定 r 是否为 East Asian Width W/F（全角）字符。
// 覆盖汉字、平假名、片假名、谚文、全角字母数字与符号等常见区间。
func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK 部首、康熙字典部首等
		return true
	case r >= 0x3041 && r <= 0x33FF: // 平假名、片假名、CJK 符号与标点、半角全角
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK 扩展 A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK 统一表意文字
		return true
	case r >= 0xA000 && r <= 0xA4CF: // 彝文
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // 谚文音节
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK 兼容表意文字
		return true
	case r >= 0xFE30 && r <= 0xFE4F: // CJK 兼容形式
		return true
	case r >= 0xFF00 && r <= 0xFF60: // 全角形式
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // 全角符号
		return true
	case r >= 0x20000 && r <= 0x3FFFD: // CJK 扩展 B 及以后
		return true
	}
	return false
}
