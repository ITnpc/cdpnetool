// 规则相关类型定义，与后端 pkg/rulespec 保持一致

// ========== 基础类型 ==========

export type RuleID = string

export type ConditionType = 
  | 'url' 
  | 'method' 
  | 'header' 
  | 'query' 
  | 'cookie' 
  | 'json_pointer' 
  | 'text' 
  | 'mime' 
  | 'size' 
  | 'probability'
  | 'stage'
  | 'time_window'

export type ConditionMode = 'prefix' | 'regex' | 'exact'

export type ConditionOp = 'equals' | 'contains' | 'regex' | 'lt' | 'lte' | 'gt' | 'gte' | 'between'

export type RuleMode = 'short_circuit' | 'aggregate'

export type PauseStage = 'request' | 'response'

export type PauseDefaultActionType = 'continue_original' | 'continue_mutated' | 'fulfill' | 'fail'

export type JSONPatchOpType = 'add' | 'remove' | 'replace' | 'move' | 'copy' | 'test'

// ========== Condition 条件 ==========

export interface Condition {
  type: ConditionType
  mode?: ConditionMode      // url, mime
  pattern?: string          // url, mime
  values?: string[]         // method
  key?: string              // header, query, cookie
  op?: ConditionOp          // header, query, cookie, text, json_pointer, size
  value?: string            // header, query, cookie, text, json_pointer, size, probability, stage
  pointer?: string          // json_pointer
}

// ========== Match 匹配器 ==========

export interface Match {
  allOf?: Condition[]
  anyOf?: Condition[]
  noneOf?: Condition[]
}

// ========== Action 动作 ==========

export interface JSONPatchOp {
  op: JSONPatchOpType
  path: string
  from?: string
  value?: any
}

export interface TextRegexPatch {
  pattern: string
  replace: string
}

export interface Base64Patch {
  value: string
}

export interface BodyPatch {
  jsonPatch?: JSONPatchOp[]
  textRegex?: TextRegexPatch
  base64?: Base64Patch
}

export interface Rewrite {
  url?: string
  method?: string
  headers?: Record<string, string | null>
  query?: Record<string, string | null>
  cookies?: Record<string, string | null>
  body?: BodyPatch
}

export interface Respond {
  status: number
  headers?: Record<string, string>
  body?: string
  base64?: boolean
}

export interface Fail {
  reason: string
}

export interface PauseDefaultAction {
  type: PauseDefaultActionType
  status?: number
  reason?: string
}

export interface Pause {
  stage: PauseStage
  timeoutMS: number
  defaultAction: PauseDefaultAction
}

export interface Action {
  rewrite?: Rewrite
  respond?: Respond
  fail?: Fail
  delayMS?: number
  dropRate?: number
  pause?: Pause
}

// ========== Rule 规则 ==========

export interface Rule {
  id: RuleID
  name?: string  // 可选规则名称
  priority: number
  mode: RuleMode
  match: Match
  action: Action
}

export interface RuleSet {
  version: string
  rules: Rule[]
}

// ========== 辅助函数 ==========

export function createEmptyCondition(type: ConditionType = 'url'): Condition {
  switch (type) {
    case 'url':
      return { type: 'url', mode: 'prefix', pattern: '' }
    case 'method':
      return { type: 'method', values: ['GET'] }
    case 'header':
    case 'query':
    case 'cookie':
      return { type, key: '', op: 'equals', value: '' }
    case 'json_pointer':
      return { type: 'json_pointer', pointer: '', op: 'equals', value: '' }
    case 'text':
      return { type: 'text', op: 'contains', value: '' }
    case 'mime':
      return { type: 'mime', mode: 'prefix', pattern: 'application/json' }
    case 'size':
      return { type: 'size', op: 'lt', value: '1048576' }
    case 'probability':
      return { type: 'probability', value: '1.0' }
    case 'stage':
      return { type: 'stage', value: 'request' }
    case 'time_window':
      return { type: 'time_window', value: '09:00-18:00' }
    default:
      return { type: 'url', mode: 'prefix', pattern: '' }
  }
}

export function createEmptyRule(): Rule {
  return {
    id: `rule_${Date.now()}`,
    priority: 100,
    mode: 'short_circuit',
    match: {
      allOf: [createEmptyCondition('url')]
    },
    action: {}
  }
}

export function createEmptyRuleSet(): RuleSet {
  return {
    version: '1.0',
    rules: []
  }
}

// 条件类型的中文标签
export const CONDITION_TYPE_LABELS: Record<ConditionType, string> = {
  url: 'URL',
  method: '请求方法',
  header: '请求头',
  query: 'Query参数',
  cookie: 'Cookie',
  json_pointer: 'JSON路径',
  text: '文本内容',
  mime: 'MIME类型',
  size: '体积大小',
  probability: '概率采样',
  stage: '拦截阶段',
  time_window: '时间窗口'
}

// 操作符的中文标签
export const CONDITION_OP_LABELS: Record<ConditionOp, string> = {
  equals: '等于',
  contains: '包含',
  regex: '正则匹配',
  lt: '小于',
  lte: '小于等于',
  gt: '大于',
  gte: '大于等于',
  between: '范围内'
}

// 模式的中文标签
export const CONDITION_MODE_LABELS: Record<ConditionMode, string> = {
  prefix: '前缀匹配',
  regex: '正则匹配',
  exact: '精确匹配'
}

// HTTP 方法选项
export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']
