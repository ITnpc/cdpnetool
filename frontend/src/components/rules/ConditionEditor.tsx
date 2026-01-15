import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { X, Plus } from 'lucide-react'
import type { Condition, ConditionType } from '@/types/rules'
import {
  CONDITION_TYPE_SHORT_LABELS,
  CONDITION_GROUPS,
  HTTP_METHODS,
  RESOURCE_TYPES,
  createEmptyCondition,
  getConditionFields
} from '@/types/rules'

interface ConditionEditorProps {
  condition: Condition
  onChange: (condition: Condition) => void
  onRemove: () => void
}

// 条件类型选项（扁平化列表）
const conditionTypeOptions: { value: ConditionType; label: string }[] = [
  // URL
  ...CONDITION_GROUPS.url.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
  // 方法/资源
  ...CONDITION_GROUPS.method.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
  ...CONDITION_GROUPS.resourceType.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
  // Header
  ...CONDITION_GROUPS.header.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
  // Query
  ...CONDITION_GROUPS.query.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
  // Cookie
  ...CONDITION_GROUPS.cookie.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
  // Body
  ...CONDITION_GROUPS.body.map(t => ({ value: t as ConditionType, label: CONDITION_TYPE_SHORT_LABELS[t] })),
]

export function ConditionEditor({ condition, onChange, onRemove }: ConditionEditorProps) {
  const handleTypeChange = (newType: ConditionType) => {
    onChange(createEmptyCondition(newType))
  }

  const updateField = <K extends keyof Condition>(key: K, value: Condition[K]) => {
    onChange({ ...condition, [key]: value })
  }

  const fields = getConditionFields(condition.type)

  return (
    <div className="flex items-start gap-2 p-3 rounded-lg border bg-card">
      {/* 条件类型选择 */}
      <Select
        value={condition.type}
        onChange={(e) => handleTypeChange(e.target.value as ConditionType)}
        options={conditionTypeOptions}
        className="w-32 shrink-0"
      />

      {/* 根据条件类型渲染字段 */}
      <div className="flex-1 flex items-center gap-2 flex-wrap">
        {renderConditionFields(condition, fields, updateField)}
      </div>

      {/* 删除按钮 */}
      <Button variant="ghost" size="icon" onClick={onRemove} className="shrink-0">
        <X className="w-4 h-4" />
      </Button>
    </div>
  )
}

// 渲染条件字段
function renderConditionFields(
  condition: Condition,
  fields: ReturnType<typeof getConditionFields>,
  updateField: <K extends keyof Condition>(key: K, value: Condition[K]) => void
) {
  const { type } = condition

  // Method 多选
  if (type === 'method') {
    return (
      <MultiValueSelector
        values={condition.values || []}
        options={[...HTTP_METHODS]}
        onChange={(values) => updateField('values', values)}
      />
    )
  }

  // ResourceType 多选
  if (type === 'resourceType') {
    return (
      <MultiValueSelector
        values={condition.values || []}
        options={[...RESOURCE_TYPES]}
        onChange={(values) => updateField('values', values)}
      />
    )
  }

  // 其他条件类型
  return (
    <>
      {/* name 字段 */}
      {fields.includes('name') && (
        <Input
          value={condition.name || ''}
          onChange={(e) => updateField('name', e.target.value)}
          placeholder={getNamePlaceholder(type)}
          className="w-32"
        />
      )}

      {/* path 字段 (bodyJsonPath) */}
      {fields.includes('path') && (
        <Input
          value={condition.path || ''}
          onChange={(e) => updateField('path', e.target.value)}
          placeholder="$.data.status"
          className="w-40"
        />
      )}

      {/* value 字段 */}
      {fields.includes('value') && (
        <Input
          value={condition.value || ''}
          onChange={(e) => updateField('value', e.target.value)}
          placeholder={getValuePlaceholder(type)}
          className="flex-1 min-w-[150px]"
        />
      )}

      {/* pattern 字段 */}
      {fields.includes('pattern') && (
        <Input
          value={condition.pattern || ''}
          onChange={(e) => updateField('pattern', e.target.value)}
          placeholder="正则表达式..."
          className="flex-1 min-w-[150px]"
        />
      )}
    </>
  )
}

// 获取 name 字段占位符
function getNamePlaceholder(type: ConditionType): string {
  if (type.startsWith('header')) return 'Header 名'
  if (type.startsWith('query')) return '参数名'
  if (type.startsWith('cookie')) return 'Cookie 名'
  return '名称'
}

// 获取 value 字段占位符
function getValuePlaceholder(type: ConditionType): string {
  if (type.startsWith('url')) return 'URL...'
  if (type === 'bodyContains') return '包含的文本...'
  if (type === 'bodyJsonPath') return '期望值'
  return '值...'
}

// 多值选择器组件
function MultiValueSelector({
  values,
  options,
  onChange
}: {
  values: string[]
  options: string[]
  onChange: (values: string[]) => void
}) {
  const toggleValue = (value: string) => {
    if (values.includes(value)) {
      onChange(values.filter(v => v !== value))
    } else {
      onChange([...values, value])
    }
  }

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {options.map(option => (
        <Badge
          key={option}
          variant={values.includes(option) ? 'default' : 'outline'}
          className="cursor-pointer select-none"
          onClick={() => toggleValue(option)}
        >
          {option}
        </Badge>
      ))}
    </div>
  )
}

// ==================== 条件组编辑器 ====================

interface ConditionGroupProps {
  title: string
  description: string
  conditions: Condition[]
  onChange: (conditions: Condition[]) => void
}

export function ConditionGroup({ title, description, conditions, onChange }: ConditionGroupProps) {
  const addCondition = () => {
    onChange([...conditions, createEmptyCondition('urlPrefix')])
  }

  const updateCondition = (index: number, condition: Condition) => {
    const newConditions = [...conditions]
    newConditions[index] = condition
    onChange(newConditions)
  }

  const removeCondition = (index: number) => {
    onChange(conditions.filter((_, i) => i !== index))
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="font-medium">{title}</h4>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>
        <Button variant="outline" size="sm" onClick={addCondition}>
          <Plus className="w-4 h-4 mr-1" />
          添加条件
        </Button>
      </div>

      {conditions.length === 0 ? (
        <div className="text-sm text-muted-foreground p-3 border rounded-lg border-dashed text-center">
          暂无条件，点击上方按钮添加
        </div>
      ) : (
        <div className="space-y-2">
          {conditions.map((condition, index) => (
            <ConditionEditor
              key={index}
              condition={condition}
              onChange={(c) => updateCondition(index, c)}
              onRemove={() => removeCondition(index)}
            />
          ))}
        </div>
      )}
    </div>
  )
}
