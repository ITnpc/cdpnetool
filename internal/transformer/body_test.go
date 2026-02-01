package transformer_test

import (
	"testing"

	"cdpnetool/internal/transformer"
	"cdpnetool/pkg/rulespec"
)

func TestReplaceText(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		search  string
		replace string
		all     bool
		want    string
	}{
		{"替换单次", "hello world hello", "hello", "hi", false, "hi world hello"},
		{"替换全部", "hello world hello", "hello", "hi", true, "hi world hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformer.ReplaceText(tt.body, tt.search, tt.replace, tt.all)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatchJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		patches []rulespec.JSONPatchOp
		want    string
		wantErr bool
	}{
		{
			name: "添加字段",
			body: `{"name":"test"}`,
			patches: []rulespec.JSONPatchOp{
				{Op: "add", Path: "/age", Value: 18},
			},
			want:    `{"name":"test","age":18}`,
			wantErr: false,
		},
		{
			name: "替换字段",
			body: `{"name":"test"}`,
			patches: []rulespec.JSONPatchOp{
				{Op: "replace", Path: "/name", Value: "new"},
			},
			want:    `{"name":"new"}`,
			wantErr: false,
		},
		{
			name: "删除字段",
			body: `{"name":"test","age":18}`,
			patches: []rulespec.JSONPatchOp{
				{Op: "remove", Path: "/age"},
			},
			want:    `{"name":"test"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformer.PatchJSON(tt.body, tt.patches)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetFormUrlencoded(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		key   string
		value string
		want  string
	}{
		{"添加字段", "name=test", "age", "18", "age=18&name=test"},
		{"覆盖字段", "name=test&age=10", "age", "18", "age=18&name=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformer.SetFormUrlencoded(tt.body, tt.key, tt.value)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveFormUrlencoded(t *testing.T) {
	tests := []struct {
		name string
		body string
		key  string
		want string
	}{
		{"删除存在字段", "name=test&age=18", "age", "name=test"},
		{"删除不存在字段", "name=test", "age", "name=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformer.RemoveFormUrlencoded(tt.body, tt.key)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeBody(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		encoding rulespec.BodyEncoding
		want     string
		wantErr  bool
	}{
		{"plain文本", "hello", "", "hello", false},
		{"base64解码", "aGVsbG8=", rulespec.BodyEncodingBase64, "hello", false},
		{"无效base64", "invalid!!!", rulespec.BodyEncodingBase64, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformer.DecodeBody(tt.input, tt.encoding)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
