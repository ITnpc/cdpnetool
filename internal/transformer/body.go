package transformer

import (
	"encoding/base64"
	"net/url"
	"strings"

	"cdpnetool/pkg/rulespec"

	"github.com/tidwall/sjson"
)

// ReplaceText 文本替换
func ReplaceText(body string, search, replace string, all bool) string {
	if all {
		return strings.ReplaceAll(body, search, replace)
	}
	return strings.Replace(body, search, replace, 1)
}

// PatchJSON 应用 JSON Patch 修改 (基于 sjson)
func PatchJSON(body string, patches []rulespec.JSONPatchOp) (string, error) {
	if body == "" || len(patches) == 0 {
		return body, nil
	}

	currentBody := body
	for _, patch := range patches {
		if patch.Path == "" {
			continue
		}

		// 将 JSON Patch 路径 (/a/b/c) 转换为 sjson 路径 (a.b.c)
		path := strings.TrimPrefix(patch.Path, "/")
		path = strings.ReplaceAll(path, "/", ".")

		var err error
		switch patch.Op {
		case "add", "replace":
			currentBody, err = sjson.Set(currentBody, path, patch.Value)
		case "remove":
			currentBody, err = sjson.Delete(currentBody, path)
		}
		if err != nil {
			return body, err
		}
	}
	return currentBody, nil
}

// SetFormUrlencoded 设置 x-www-form-urlencoded 字段
func SetFormUrlencoded(body, key, value string) (string, error) {
	values, err := url.ParseQuery(body)
	if err != nil {
		return body, err
	}
	values.Set(key, value)
	return values.Encode(), nil
}

// RemoveFormUrlencoded 移除 x-www-form-urlencoded 字段
func RemoveFormUrlencoded(body, key string) (string, error) {
	values, err := url.ParseQuery(body)
	if err != nil {
		return body, err
	}
	values.Del(key)
	return values.Encode(), nil
}

// DecodeBody 根据编码方式解码
func DecodeBody(input string, encoding rulespec.BodyEncoding) (string, error) {
	if encoding == rulespec.BodyEncodingBase64 {
		decoded, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}
	return input, nil
}
