// Package httpapi 提供 Markdown 表格生成与格式化服务的 HTTP 接口。
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"task017-mdtable/internal/mdtable"
)

// ErrBadJSON 表示请求体不是单个合法 JSON 对象。
var ErrBadJSON = errors.New("请求体不是合法的单个 JSON 对象")

// API 是 Markdown 表格服务的 HTTP 接口实现。服务无内部可变状态，
// 可被多个 goroutine 复用。
type API struct{}

// New 创建服务实例。
func New() *API { return &API{} }

// Handler 返回 HTTP 路由。
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /format", a.format)
	mux.HandleFunc("POST /validate", a.validate)
	mux.HandleFunc("POST /width", a.width)
	return mux
}

// decodeJSON 解码单个 JSON 对象，拒绝多段 JSON 与未知字段。
func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return ErrBadJSON
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", ErrBadJSON, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return ErrBadJSON
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type formatRequest struct {
	Header []string   `json:"header"`
	Rows   [][]string `json:"rows"`
	Aligns []string   `json:"aligns"` // 可省略；省略时全部默认对齐
}

type formatResponse struct {
	Table  string `json:"table"`
	Widths []int  `json:"widths"`
}

func (a *API) format(w http.ResponseWriter, r *http.Request) {
	var req formatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "status": http.StatusBadRequest})
		return
	}
	aligns, err := parseAligns(req.Aligns, len(req.Header))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "status": http.StatusBadRequest})
		return
	}
	table, widths, err := mdtable.Format(req.Header, req.Rows, aligns)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "status": http.StatusBadRequest})
		return
	}
	writeJSON(w, http.StatusOK, formatResponse{Table: table, Widths: widths})
}

type validateResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (a *API) validate(w http.ResponseWriter, r *http.Request) {
	var req formatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, validateResponse{OK: false, Error: err.Error()})
		return
	}
	aligns, err := parseAligns(req.Aligns, len(req.Header))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, validateResponse{OK: false, Error: err.Error()})
		return
	}
	if err := mdtable.Validate(req.Header, req.Rows, aligns); err != nil {
		writeJSON(w, http.StatusOK, validateResponse{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, validateResponse{OK: true})
}

type widthRequest struct {
	Text string `json:"text"`
}

type widthResponse struct {
	Width int `json:"width"`
}

func (a *API) width(w http.ResponseWriter, r *http.Request) {
	var req widthRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "status": http.StatusBadRequest})
		return
	}
	writeJSON(w, http.StatusOK, widthResponse{Width: mdtable.DisplayWidth(req.Text)})
}

// parseAligns 把字符串对齐方式数组转为 mdtable.Alignment 数组。
// aligns 为 nil（JSON 字段省略或显式 null）时返回 nil，表示全部默认对齐；
// 非 nil 时长度必须等于 ncols，且每个取值合法，否则返回错误。
func parseAligns(aligns []string, ncols int) ([]mdtable.Alignment, error) {
	if aligns == nil {
		return nil, nil
	}
	if len(aligns) != ncols {
		return nil, fmt.Errorf("对齐方式数组长度 %d 与表头列数 %d 不一致", len(aligns), ncols)
	}
	out := make([]mdtable.Alignment, len(aligns))
	for i, s := range aligns {
		a, err := mdtable.ParseAlignment(s)
		if err != nil {
			return nil, fmt.Errorf("第 %d 列%v", i, err)
		}
		out[i] = a
	}
	return out, nil
}
