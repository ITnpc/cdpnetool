import { useState, useMemo } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ChevronDown, ChevronUp, Trash2, GripVertical, Power, PowerOff } from 'lucide-react'
import { ConditionGroup } from './ConditionEditor'
import { ActionsEditor } from './ActionEditor'
import type { Rule, Action, Condition } from '@/types/rules'
import { isTerminalAction, getActionTypeLabel } from '@/types/rules'
import { useTranslation } from 'react-i18next'

type MatchMode = 'allOf' | 'anyOf'

interface RuleEditorProps {
  rule: Rule
  onChange: (rule: Rule) => void
  onRemove: () => void
  isExpanded?: boolean
  onToggleExpand?: () => void
}

export function RuleEditor({
  rule,
  onChange,
  onRemove,
  isExpanded = true,
  onToggleExpand
}: RuleEditorProps) {
  const { t } = useTranslation()
  const [conditionsExpanded, setConditionsExpanded] = useState(true)
  const [actionsExpanded, setActionsExpanded] = useState(true)

  // 确定当前匹配模式
  const currentMode: MatchMode = useMemo(() => {
    const hasAllOf = rule.match.allOf !== undefined && rule.match.allOf !== null
    const hasAnyOf = rule.match.anyOf !== undefined && rule.match.anyOf !== null
    
    if (hasAllOf) return 'allOf'
    if (hasAnyOf) return 'anyOf'
    return 'allOf' // 默认
  }, [rule.match.allOf, rule.match.anyOf])

  const currentConditions = currentMode === 'allOf' 
    ? (rule.match.allOf || []) 
    : (rule.match.anyOf || [])

  const handleModeChange = (newMode: MatchMode) => {
    if (newMode === currentMode) return
    
    // 将条件从旧模式迁移到新模式
    const conditions = currentConditions.length > 0 ? currentConditions : []
    onChange({
      ...rule,
      match: {
        allOf: newMode === 'allOf' ? conditions : undefined,
        anyOf: newMode === 'anyOf' ? conditions : undefined
      }
    })
  }

  const handleConditionsChange = (conditions: Condition[]) => {
    onChange({
      ...rule,
      match: {
        allOf: currentMode === 'allOf' ? conditions : undefined,
        anyOf: currentMode === 'anyOf' ? conditions : undefined
      }
    })
  }

  const updateActions = (actions: Action[]) => {
    onChange({ ...rule, actions })
  }

  const toggleEnabled = (e: React.MouseEvent) => {
    e.stopPropagation()
    onChange({ ...rule, enabled: !rule.enabled })
  }

  // 计算条件数量摘要
  const conditionCount = useMemo(
    () => (rule.match.allOf?.length || 0) + (rule.match.anyOf?.length || 0),
    [rule.match.allOf, rule.match.anyOf]
  )

  // 获取行为摘要
  const actionsSummary = useMemo(() => {
    if (rule.actions.length === 0) return 'No actions'
    const types = rule.actions.map(a => getActionTypeLabel(a.type))
    if (types.length <= 2) return types.join(', ')
    return `${types[0]} & ${types.length - 1} more`
  }, [rule.actions])

  // 检查是否有终结性行为
  const hasTerminalAction = useMemo(
    () => rule.actions.some(isTerminalAction),
    [rule.actions]
  )

  return (
    <div className={`border rounded-lg bg-card overflow-hidden ${!rule.enabled ? 'opacity-60' : ''}`}>
      {/* 折叠头部 */}
      <div
        className="flex items-center gap-2 px-3 py-2 bg-muted/50 cursor-pointer hover:bg-muted/70 transition-colors"
        onClick={onToggleExpand}
      >
        <GripVertical className="w-4 h-4 text-muted-foreground cursor-grab" />

        {/* 启用/禁用按钮 */}
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={toggleEnabled}
          title={rule.enabled ? t('rules.enabled') : t('rules.disabled')}
        >
          {rule.enabled ? (
            <Power className="w-3.5 h-3.5 text-green-500" />
          ) : (
            <PowerOff className="w-3.5 h-3.5 text-muted-foreground" />
          )}
        </Button>

        <div className="flex-1 min-w-0 flex items-center gap-2">
          <span className="font-medium text-sm truncate">{rule.name || rule.id}</span>
          <Badge variant="secondary" className="text-xs shrink-0">
            {t('rules.priority')} {rule.priority}
          </Badge>
          {hasTerminalAction && (
            <Badge variant="destructive" className="text-xs shrink-0">
              {t('rules.terminal')}
            </Badge>
          )}
          <span className="text-xs text-muted-foreground shrink-0">
            {conditionCount} {t('rules.conditions')} · {actionsSummary}
          </span>
        </div>

        <div className="flex items-center gap-1 shrink-0">
          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={(e) => { e.stopPropagation(); onRemove() }}>
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
          {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </div>
      </div>

      {/* 展开内容 */}
      {isExpanded && (
        <div className="p-4 space-y-4">
          {/* 基础信息 */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <div className="space-y-1">
              <label className="text-sm font-medium">ID</label>
              <Input
                value={rule.id}
                onChange={(e) => onChange({ ...rule, id: e.target.value })}
                placeholder="ID"
              />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">Name</label>
              <Input
                value={rule.name}
                onChange={(e) => onChange({ ...rule, name: e.target.value })}
                placeholder="Name"
              />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium">{t('rules.priority')}</label>
              <Input
                type="number"
                value={rule.priority}
                onChange={(e) => onChange({ ...rule, priority: parseInt(e.target.value) || 0 })}
                placeholder="0"
              />
            </div>
          </div>

          {/* 匹配条件 */}
          <div className="space-y-4">
            <div 
              className="relative flex items-center cursor-pointer group"
              onClick={() => setConditionsExpanded(!conditionsExpanded)}
            >
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-semibold whitespace-nowrap">{t('rules.conditions')} ({conditionCount})</h3>
                {conditionsExpanded ? (
                  <ChevronUp className="w-4 h-4 text-muted-foreground" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-muted-foreground" />
                )}
              </div>
              <div className="flex-1 ml-3 border-t border-dashed border-border"></div>
            </div>
            {conditionsExpanded && (
              <div className="space-y-4">
                {/* 条件列表 */}
                <ConditionGroup
                  title=""
                  description=""
                  conditions={currentConditions}
                  onChange={handleConditionsChange}
                  headerSlot={
                    <div className="flex items-center gap-3">
                      <div className="flex items-center bg-muted rounded-md p-0.5">
                        <button
                          className={`px-3 py-1 text-xs font-medium rounded transition-all ${
                            currentMode === 'allOf' 
                              ? 'bg-background text-foreground shadow-sm' 
                              : 'text-muted-foreground hover:text-foreground'
                          }`}
                          onClick={() => handleModeChange('allOf')}
                        >
                          {t('rules.allOf')}
                        </button>
                        <button
                          className={`px-3 py-1 text-xs font-medium rounded transition-all ${
                            currentMode === 'anyOf' 
                              ? 'bg-background text-foreground shadow-sm' 
                              : 'text-muted-foreground hover:text-foreground'
                          }`}
                          onClick={() => handleModeChange('anyOf')}
                        >
                          {t('rules.anyOf')}
                        </button>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {currentMode === 'allOf' ? t('rules.allOfDesc') : t('rules.anyOfDesc')}
                      </span>
                    </div>
                  }
                />
              </div>
            )}
          </div>

          {/* 执行行为 */}
          <div className="space-y-4">
            <div 
              className="relative flex items-center cursor-pointer group"
              onClick={() => setActionsExpanded(!actionsExpanded)}
            >
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-semibold whitespace-nowrap">{t('rules.actions')} ({rule.actions.length})</h3>
                {actionsExpanded ? (
                  <ChevronUp className="w-4 h-4 text-muted-foreground" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-muted-foreground" />
                )}
              </div>
              <div className="flex-1 ml-3 border-t border-dashed border-border"></div>
            </div>
            {actionsExpanded && (
              <ActionsEditor
                actions={rule.actions}
                onChange={updateActions}
                stage={rule.stage}
                onStageChange={(stage) => onChange({ ...rule, stage, actions: [] })}
              />
            )}
          </div>
        </div>
      )}
    </div>
  )
}

interface RuleListEditorProps {
  rules: Rule[]
  onChange: (rules: Rule[]) => void
}

export function RuleListEditor({ rules, onChange }: RuleListEditorProps) {
  const { t } = useTranslation()
  const [expandedRules, setExpandedRules] = useState<Set<string>>(new Set())

  const toggleExpand = (ruleId: string) => {
    const newSet = new Set(expandedRules)
    if (newSet.has(ruleId)) {
      newSet.delete(ruleId)
    } else {
      newSet.add(ruleId)
    }
    setExpandedRules(newSet)
  }

  const updateRule = (index: number, rule: Rule) => {
    const newRules = [...rules]
    newRules[index] = rule
    onChange(newRules)
  }

  const removeRule = (index: number) => {
    onChange(rules.filter((_, i) => i !== index))
  }

  // 按优先级从大到小排序展示
  const sortedRules = [...rules].sort((a, b) => b.priority - a.priority)

  return (
    <div className="space-y-3">
      {sortedRules.map((rule) => {
        const originalIndex = rules.findIndex(r => r.id === rule.id)
        return (
          <RuleEditor
            key={rule.id}
            rule={rule}
            onChange={(r) => updateRule(originalIndex, r)}
            onRemove={() => removeRule(originalIndex)}
            isExpanded={expandedRules.has(rule.id)}
            onToggleExpand={() => toggleExpand(rule.id)}
          />
        )
      })}

      {rules.length === 0 && (
        <div className="text-center text-muted-foreground p-8 border rounded-lg border-dashed">
          {t('rules.noRules')}
        </div>
      )}
    </div>
  )
}