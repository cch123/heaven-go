// Package unityyaml 提供 Unity 序列化 YAML 的最小解析支持。
//
// Unity 资产文件是多文档 YAML，文档标记形如：
//
//	--- !u!74 &7400000          （classID=74 AnimationClip，fileID=7400000）
//	--- !u!4  &123 stripped     （嵌套 prefab 的剥离引用，尾缀非法 YAML，需剔除）
//
// 本包把每个文档解析为 map[string]any，并提供宽松的数值/字段访问辅助
// （Unity 会把无穷斜率写成字符串 "Infinity"）。
package unityyaml

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc 是一个 Unity YAML 文档。
type Doc struct {
	ClassID  int   // Unity class，如 1=GameObject 4=Transform 74=AnimationClip 95=Animator 212=SpriteRenderer
	FileID   int64 // 文档内引用锚
	Stripped bool  // 嵌套 prefab 的剥离引用文档
	Root     map[string]any
}

var docMarker = regexp.MustCompile(`(?m)^--- !u!(\d+) &(-?\d+)( stripped)?\s*$`)

// Parse 解析多文档 Unity YAML。
func Parse(data []byte) ([]Doc, error) {
	text := string(data)
	idx := docMarker.FindAllStringSubmatchIndex(text, -1)
	if len(idx) == 0 {
		return nil, fmt.Errorf("no unity yaml document markers found")
	}

	docs := make([]Doc, 0, len(idx))
	for k, m := range idx {
		classID, _ := strconv.Atoi(text[m[2]:m[3]])
		fileID, _ := strconv.ParseInt(text[m[4]:m[5]], 10, 64)

		bodyStart := m[1]
		bodyEnd := len(text)
		if k+1 < len(idx) {
			bodyEnd = idx[k+1][0]
		}
		var root map[string]any
		if err := yaml.Unmarshal([]byte(text[bodyStart:bodyEnd]), &root); err != nil {
			return nil, fmt.Errorf("doc !u!%d &%d: %w", classID, fileID, err)
		}
		docs = append(docs, Doc{ClassID: classID, FileID: fileID, Stripped: m[6] >= 0, Root: root})
	}
	return docs, nil
}

// ParseSingle 解析单文档 YAML（如 .meta 文件）。
func ParseSingle(data []byte) (map[string]any, error) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return root, nil
}

// Content 返回文档根下唯一对象的内容（如 Root["AnimationClip"]）。
func (d *Doc) Content() map[string]any {
	for _, v := range d.Root {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// Kind 返回文档根键名（如 "AnimationClip"）。
func (d *Doc) Kind() string {
	for k := range d.Root {
		return k
	}
	return ""
}

// ---------- 宽松访问辅助 ----------

// M 取子 map；任何不匹配返回 nil。
func M(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// L 取子列表。
func L(v any) []any {
	l, _ := v.([]any)
	return l
}

// Get 按 key 路径取值。对 "y"/"n" 这类可能被 YAML 1.1 解析为布尔的键做了回退。
func Get(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm := M(cur)
		if mm == nil {
			return nil
		}
		v, ok := mm[k]
		if !ok {
			switch k { // yaml 解析器把裸 y/n 键解析为布尔字符串时的回退
			case "y":
				v, ok = mm["true"]
			case "n":
				v, ok = mm["false"]
			}
			if !ok {
				return nil
			}
		}
		cur = v
	}
	return cur
}

// F 宽松转 float64："Infinity"/"-Infinity" 转 ±Inf。
func F(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	case string:
		s := strings.TrimSpace(n)
		switch s {
		case "Infinity":
			return math.Inf(1)
		case "-Infinity":
			return math.Inf(-1)
		}
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return f
		}
	}
	return 0
}

// I 宽松转 int64。
func I(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

// S 宽松转 string。
func S(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
