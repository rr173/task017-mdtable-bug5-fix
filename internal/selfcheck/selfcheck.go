// Package selfcheck 提供无需外部依赖的自检：通过 httptest 启动真实 HTTP
// 服务，覆盖格式化、校验、宽度端点与各类边界约束。成功返回 0，任一失败返回 1。
package selfcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"task017-mdtable/internal/httpapi"
)

// Run 执行自检并返回退出码。
func Run() int {
	passed, failed := 0, 0
	check := func(name string, fn func() error) {
		if err := fn(); err != nil {
			failed++
			fmt.Printf("FAIL %-36s %v\n", name, err)
		} else {
			passed++
			fmt.Printf("PASS %s\n", name)
		}
	}

	srv := httptest.NewServer(httpapi.New().Handler())
	defer srv.Close()

	do := func(method, path, body string) (*http.Response, []byte, error) {
		var r io.Reader
		if body != "" {
			r = bytes.NewReader([]byte(body))
		}
		req, err := http.NewRequest(method, srv.URL+path, r)
		if err != nil {
			return nil, nil, err
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, data, readErr
	}

	format := func(body string) (int, string, []byte, error) {
		resp, data, err := do(http.MethodPost, "/format", body)
		if err != nil {
			return 0, "", nil, err
		}
		var out struct {
			Table  string `json:"table"`
			Widths []int  `json:"widths"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal(data, &out)
		return resp.StatusCode, out.Table, data, nil
	}

	validate := func(body string) (int, bool, string, error) {
		resp, data, err := do(http.MethodPost, "/validate", body)
		if err != nil {
			return 0, false, "", err
		}
		var out struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &out)
		return resp.StatusCode, out.OK, out.Error, nil
	}

	check("健康检查", func() error {
		resp, _, err := do(http.MethodGet, "/healthz", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	check("基本格式化与列宽", func() error {
		body := `{"header":["Name","Age"],"rows":[["Alice","30"],["Bob","25"]],"aligns":["left","right"]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		want := "| Name  | Age |\n| :---- | --: |\n| Alice |  30 |\n| Bob   |  25 |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		return nil
	})

	check("默认对齐(省略 aligns)", func() error {
		body := `{"header":["a","b"],"rows":[["1","2"]]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		want := "| a   | b   |\n| --- | --- |\n| 1   | 2   |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		return nil
	})

	check("竖线转义保结构", func() error {
		body := `{"header":["a","b"],"rows":[["x|y","z"]]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		if !strings.Contains(table, `x\|y`) {
			return fmt.Errorf("pipe not escaped: %q", table)
		}
		for _, line := range strings.Split(strings.TrimRight(table, "\n"), "\n") {
			if n := countColumns(line); n != 2 {
				return fmt.Errorf("line %q has %d columns, want 2", line, n)
			}
		}
		return nil
	})

	check("反斜杠先于竖线转义", func() error {
		// JSON 中 \\ 表示一个字面反斜杠；单元格内容为 a\b|c。
		body := `{"header":["a"],"rows":[["a\\b|c"]]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		if !strings.Contains(table, `a\\b\|c`) {
			return fmt.Errorf("escape mismatch: %q", table)
		}
		return nil
	})

	check("表头含换行被拒", func() error {
		body := `{"header":["a\nb"]}`
		status, _, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("单元格含换行被拒", func() error {
		body := `{"header":["a"],"rows":[["x\ny"]]}`
		status, _, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("CJK 显示宽度对齐", func() error {
		body := `{"header":["项目","数量"],"rows":[["苹果","5"],["香蕉","12"]]}`
		status, table, raw, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", status, raw)
		}
		want := "| 项目 | 数量 |\n| ---- | ---- |\n| 苹果 | 5    |\n| 香蕉 | 12   |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		var out struct {
			Widths []int `json:"widths"`
		}
		_ = json.Unmarshal(raw, &out)
		if len(out.Widths) != 2 || out.Widths[0] != 4 || out.Widths[1] != 4 {
			return fmt.Errorf("widths=%v want [4 4]", out.Widths)
		}
		return nil
	})

	check("数据行多于表头被拒", func() error {
		body := `{"header":["a","b"],"rows":[["1","2","3"]]}`
		status, _, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("数据行少于表头补齐", func() error {
		body := `{"header":["a","b","c"],"rows":[["1"]]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		want := "| a   | b   | c   |\n| --- | --- | --- |\n| 1   |     |     |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		return nil
	})

	check("表头为空被拒", func() error {
		body := `{"header":[]}`
		status, _, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("aligns 长度不匹配被拒", func() error {
		body := `{"header":["a","b"],"aligns":["left"]}`
		status, _, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("仅表头无数据行", func() error {
		body := `{"header":["h1","h2"]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		want := "| h1  | h2  |\n| --- | --- |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		return nil
	})

	check("单列表格合法", func() error {
		body := `{"header":["h"],"rows":[["x"]]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		want := "| h   |\n| --- |\n| x   |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		return nil
	})

	check("居中对齐分隔行", func() error {
		body := `{"header":["h"],"rows":[["x"]],"aligns":["center"]}`
		status, table, _, err := format(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		want := "|  h  |\n| :-: |\n|  x  |\n"
		if table != want {
			return fmt.Errorf("table mismatch:\ngot:  %q\nwant: %q", table, want)
		}
		return nil
	})

	check("宽度端点", func() error {
		cases := []struct {
			text string
			want int
		}{
			{`abc`, 3},
			{`中`, 2},
			{`a中b`, 4},
		}
		for _, c := range cases {
			body := fmt.Sprintf(`{"text":%q}`, c.text)
			resp, data, err := do(http.MethodPost, "/width", body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("width status=%d body=%s", resp.StatusCode, data)
			}
			var out struct {
				Width int `json:"width"`
			}
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			if out.Width != c.want {
				return fmt.Errorf("width(%q)=%d want %d", c.text, out.Width, c.want)
			}
		}
		return nil
	})

	check("校验端点合法输入", func() error {
		body := `{"header":["a","b"],"rows":[["1","2"]]}`
		status, ok, _, err := validate(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		if !ok {
			return fmt.Errorf("ok=false want true")
		}
		return nil
	})

	check("校验端点非法输入", func() error {
		body := `{"header":["a"],"rows":[["1","2","3"]]}`
		status, ok, errStr, err := validate(body)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d", status)
		}
		if ok {
			return fmt.Errorf("ok=true want false")
		}
		if errStr == "" {
			return fmt.Errorf("error message empty")
		}
		return nil
	})

	check("非法 JSON 被拒", func() error {
		status, _, _, err := format("{not json")
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("多段 JSON 被拒", func() error {
		status, _, _, err := format(`{"header":["a"]}{"header":["b"]}`)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("未知字段被拒", func() error {
		status, _, _, err := format(`{"header":["a"],"extra":1}`)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("非法对齐值被拒", func() error {
		status, _, _, err := format(`{"header":["a"],"aligns":["diagonal"]}`)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// countColumns 按未转义的竖线统计表格行的列数。
func countColumns(line string) int {
	var cols []string
	var cur strings.Builder
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) && runes[i+1] == '|' {
			cur.WriteRune('|')
			i++
			continue
		}
		if runes[i] == '|' {
			cols = append(cols, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(runes[i])
	}
	cols = append(cols, cur.String())
	for len(cols) >= 1 && cols[0] == "" {
		cols = cols[1:]
	}
	for len(cols) >= 1 && cols[len(cols)-1] == "" {
		cols = cols[:len(cols)-1]
	}
	return len(cols)
}
