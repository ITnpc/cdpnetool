package cdp

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"

	"cdpnetool/pkg/rulespec"
)

// applyBodyPatch 根据 BodyPatch 对文本或二进制内容进行修改
func applyBodyPatch(src string, bp *rulespec.BodyPatch) ([]byte, bool) {
	if bp == nil {
		return nil, false
	}
	// Base64 覆盖：直接以配置的 Base64 内容替换原文
	if bp.Base64 != nil {
		b, err := base64.StdEncoding.DecodeString(bp.Base64.Value)
		if err != nil {
			return nil, false
		}
		return b, true
	}
	// 文本正则替换：基于原始字符串进行正则替换
	if bp.TextRegex != nil {
		re, err := regexp.Compile(bp.TextRegex.Pattern)
		if err != nil {
			return nil, false
		}
		return []byte(re.ReplaceAllString(src, bp.TextRegex.Replace)), true
	}
	// JSON Patch：按 RFC6902 对 JSON 文本进行补丁
	if len(bp.JSONPatch) > 0 {
		out, ok := applyJSONPatch(src, bp.JSONPatch)
		if !ok {
			return nil, false
		}
		return []byte(out), true
	}
	return nil, false
}

// applyJSONPatch 对JSON文档应用Patch操作并返回结果
func applyJSONPatch(doc string, ops []rulespec.JSONPatchOp) (string, bool) {
	var v any
	if doc == "" {
		v = make(map[string]any)
	} else {
		if err := json.Unmarshal([]byte(doc), &v); err != nil {
			return "", false
		}
	}
	for _, op := range ops {
		typ := string(op.Op)
		path := op.Path
		val := op.Value
		from := op.From
		switch typ {
		case string(rulespec.JSONPatchOpAdd), string(rulespec.JSONPatchOpReplace):
			v = setByPtr(v, path, val, typ == string(rulespec.JSONPatchOpReplace))
		case string(rulespec.JSONPatchOpRemove):
			v = removeByPtr(v, path)
		case string(rulespec.JSONPatchOpCopy):
			src, ok := getByPtr(v, from)
			if !ok {
				return "", false
			}
			v = setByPtr(v, path, src, true)
		case string(rulespec.JSONPatchOpMove):
			src, ok := getByPtr(v, from)
			if !ok {
				return "", false
			}
			v = removeByPtr(v, from)
			v = setByPtr(v, path, src, true)
		case string(rulespec.JSONPatchOpTest):
			cur, ok := getByPtr(v, path)
			if !ok {
				return "", false
			}
			if !deepEqual(cur, val) {
				return "", false
			}
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// setByPtr 依据JSON Pointer设置节点值
func setByPtr(cur any, ptr string, val any, replace bool) any {
	if ptr == "" || ptr[0] != '/' {
		return cur
	}
	tokens := splitPtr(ptr)
	return setRec(cur, tokens, val)
}

// setRec 递归设置节点值的内部实现
func setRec(cur any, tokens []string, val any) any {
	if len(tokens) == 0 {
		return val
	}
	t := tokens[0]
	switch c := cur.(type) {
	case map[string]any:
		child, ok := c[t]
		if !ok {
			child = make(map[string]any)
		}
		c[t] = setRec(child, tokens[1:], val)
		return c
	case []any:
		idx, ok := toIndex(t)
		if !ok || idx < 0 || idx >= len(c) {
			return c
		}
		c[idx] = setRec(c[idx], tokens[1:], val)
		return c
	default:
		if len(tokens) == 1 {
			return val
		}
		return cur
	}
}

// removeByPtr 依据JSON Pointer移除节点
func removeByPtr(cur any, ptr string) any {
	if ptr == "" || ptr[0] != '/' {
		return cur
	}
	tokens := splitPtr(ptr)
	return removeRec(cur, tokens)
}

// getByPtr 依据JSON Pointer读取节点值
func getByPtr(cur any, ptr string) (any, bool) {
	if ptr == "" || ptr[0] != '/' {
		return nil, false
	}
	tokens := splitPtr(ptr)
	x := cur
	for _, t := range tokens {
		switch c := x.(type) {
		case map[string]any:
			v, ok := c[t]
			if !ok {
				return nil, false
			}
			x = v
		case []any:
			idx, ok := toIndex(t)
			if !ok || idx < 0 || idx >= len(c) {
				return nil, false
			}
			x = c[idx]
		default:
			return nil, false
		}
	}
	return x, true
}

// deepEqual 深度比较两个值是否相等
func deepEqual(a, b any) bool { return reflect.DeepEqual(a, b) }

// removeRec 递归移除节点的内部实现
func removeRec(cur any, tokens []string) any {
	if len(tokens) == 0 {
		return cur
	}
	t := tokens[0]
	switch c := cur.(type) {
	case map[string]any:
		if len(tokens) == 1 {
			delete(c, t)
			return c
		}
		child, ok := c[t]
		if !ok {
			return c
		}
		c[t] = removeRec(child, tokens[1:])
		return c
	case []any:
		idx, ok := toIndex(t)
		if !ok || idx < 0 || idx >= len(c) {
			return c
		}
		if len(tokens) == 1 {
			nc := append(c[:idx], c[idx+1:]...)
			return nc
		}
		c[idx] = removeRec(c[idx], tokens[1:])
		return c
	default:
		return cur
	}
}

// splitPtr 将JSON Pointer切分为令牌序列
func splitPtr(p string) []string {
	var out []string
	i := 1
	for i < len(p) {
		j := i
		for j < len(p) && p[j] != '/' {
			j++
		}
		tok := p[i:j]
		tok = strings.ReplaceAll(tok, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		out = append(out, tok)
		i = j + 1
	}
	return out
}

// toIndex 将字符串转换为数组索引
func toIndex(s string) (int, bool) {
	n := 0
	if len(s) == 0 {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// applyHeaderPatch 根据补丁对当前头部映射进行增删改
func applyHeaderPatch(cur map[string]string, patch map[string]*string) map[string]string {
	if patch == nil {
		return cur
	}
	for k, v := range patch {
		lk := strings.ToLower(k)
		if v == nil {
			delete(cur, lk)
		} else {
			cur[lk] = *v
		}
	}
	return cur
}
