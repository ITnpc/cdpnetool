package main

// 中文标签映射
var (
	actionLabels = map[string]string{
		"continue": "继续 (continue)",
		"fail":     "失败 (fail)",
		"respond":  "响应 (respond)",
		"rewrite":  "重写 (rewrite)",
		"pause":    "暂停 (pause)",
	}

	stageLabels = map[string]string{
		"request":  "请求阶段 (request)",
		"response": "响应阶段 (response)",
	}

	modeLabels = map[string]string{
		"short_circuit": "短路模式 (short_circuit)",
		"aggregate":     "聚合模式 (aggregate)",
	}

	conditionTypeLabels = map[string]string{
		"url":          "URL (url)",
		"method":       "请求方法 (method)",
		"header":       "请求头 (header)",
		"query":        "查询参数 (query)",
		"cookie":       "Cookie (cookie)",
		"text":         "文本内容 (text)",
		"mime":         "MIME类型 (mime)",
		"size":         "大小 (size)",
		"probability":  "概率 (probability)",
		"time_window":  "时间窗口 (time_window)",
		"json_pointer": "JSON指针 (json_pointer)",
		"stage":        "阶段 (stage)",
	}

	conditionOpLabels = map[string]string{
		"equals":   "等于 (equals)",
		"contains": "包含 (contains)",
		"regex":    "正则 (regex)",
		"lt":       "小于 (lt)",
		"lte":      "小于等于 (lte)",
		"gt":       "大于 (gt)",
		"gte":      "大于等于 (gte)",
		"between":  "区间 (between)",
	}

	matchGroupLabels = map[string]string{
		"allOf":  "全部满足 (allOf)",
		"anyOf":  "任一满足 (anyOf)",
		"noneOf": "全部不满足 (noneOf)",
	}
)

// getActionOptions 获取动作选项列表
func getActionOptions() []string {
	keys := []string{"continue", "fail", "respond", "rewrite", "pause"}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = actionLabels[k]
	}
	return result
}

// getStageOptions 获取阶段选项列表
func getStageOptions() []string {
	keys := []string{"request", "response"}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = stageLabels[k]
	}
	return result
}

// getModeOptions 获取模式选项列表
func getModeOptions() []string {
	keys := []string{"short_circuit", "aggregate"}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = modeLabels[k]
	}
	return result
}

// getConditionTypeOptions 获取条件类型选项列表
func getConditionTypeOptions() []string {
	keys := []string{"url", "method", "header", "query", "cookie", "text", "mime", "size", "stage"}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = conditionTypeLabels[k]
	}
	return result
}

// getConditionOpOptions 获取操作符选项列表
func getConditionOpOptions() []string {
	keys := []string{"equals", "contains", "regex", "lt", "lte", "gt", "gte"}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = conditionOpLabels[k]
	}
	return result
}

// getMatchGroupOptions 获取匹配组选项列表
func getMatchGroupOptions() []string {
	keys := []string{"allOf", "anyOf", "noneOf"}
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = matchGroupLabels[k]
	}
	return result
}

// extractValue 从带标签的选项中提取原始值
func extractValue(labeledOption string) string {
	// 格式: "中文 (value)"，提取括号中的值
	for i := len(labeledOption) - 1; i >= 0; i-- {
		if labeledOption[i] == '(' {
			if i+1 < len(labeledOption)-1 {
				return labeledOption[i+1 : len(labeledOption)-1]
			}
		}
	}
	return labeledOption
}

// findLabeledOption 根据原始值查找带标签的选项
func findLabeledOption(value string, labels map[string]string) string {
	if label, ok := labels[value]; ok {
		return label
	}
	return value
}
