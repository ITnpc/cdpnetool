import { useState, useEffect, useRef } from 'react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Toaster } from '@/components/ui/toaster'
import { useToast } from '@/hooks/use-toast'
import { useSessionStore, useThemeStore } from '@/stores'
import { RuleListEditor } from '@/components/rules'
import { EventsPanel } from '@/components/events'
import type { Rule, RuleSet } from '@/types/rules'
import type { InterceptEvent } from '@/types/events'
import { createEmptyRule, createEmptyRuleSet } from '@/types/rules'
import { 
  RefreshCw, 
  Moon, 
  Sun,
  Link2,
  Link2Off,
  FileJson,
  Activity,
  Plus,
  Download,
  Upload,
  Save,
  Chrome,
  Trash2,
  Edit3
} from 'lucide-react'

// 规则集记录类型
interface RuleSetRecord {
  id: number
  name: string
  description: string
  version: string
  rulesJson: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

interface OperationResult {
  success: boolean
  error?: string
}

// Wails 生成的绑定（需要在 wails dev 后生成）
declare global {
  interface Window {
    go: {
      gui: {
        App: {
          StartSession: (url: string) => Promise<{ sessionId: string; success: boolean; error?: string }>
          StopSession: (id: string) => Promise<{ success: boolean; error?: string }>
          ListTargets: (id: string) => Promise<{ targets: any[]; success: boolean; error?: string }>
          AttachTarget: (sid: string, tid: string) => Promise<{ success: boolean; error?: string }>
          DetachTarget: (sid: string, tid: string) => Promise<{ success: boolean; error?: string }>
          EnableInterception: (id: string) => Promise<{ success: boolean; error?: string }>
          DisableInterception: (id: string) => Promise<{ success: boolean; error?: string }>
          LoadRules: (id: string, json: string) => Promise<{ success: boolean; error?: string }>
          GetRuleStats: (id: string) => Promise<{ stats: any; success: boolean; error?: string }>
          ApproveRequest: (itemId: string, mutationsJson: string) => Promise<{ success: boolean; error?: string }>
          ApproveResponse: (itemId: string, mutationsJson: string) => Promise<{ success: boolean; error?: string }>
          Reject: (itemId: string) => Promise<{ success: boolean; error?: string }>
          LaunchBrowser: (headless: boolean) => Promise<{ devToolsUrl: string; success: boolean; error?: string }>
          CloseBrowser: () => Promise<{ success: boolean; error?: string }>
          GetBrowserStatus: () => Promise<{ devToolsUrl: string; success: boolean; error?: string }>
          // 规则集持久化 API
          ListRuleSets: () => Promise<{ ruleSets: RuleSetRecord[]; success: boolean; error?: string }>
          GetRuleSet: (id: number) => Promise<{ ruleSet: RuleSetRecord; success: boolean; error?: string }>
          SaveRuleSet: (id: number, name: string, description: string, rulesJson: string) => Promise<{ ruleSet: RuleSetRecord; success: boolean; error?: string }>
          DeleteRuleSet: (id: number) => Promise<{ success: boolean; error?: string }>
          SetActiveRuleSet: (id: number) => Promise<{ success: boolean; error?: string }>
          GetActiveRuleSet: () => Promise<{ ruleSet: RuleSetRecord | null; success: boolean; error?: string }>
          RenameRuleSet: (id: number, newName: string) => Promise<{ success: boolean; error?: string }>
          SetDirty: (dirty: boolean) => Promise<void>
          ExportRuleSet: (name: string, json: string) => Promise<OperationResult>
        }
      }
    }
  }
}

function App() {
  const { 
    devToolsURL, 
    setDevToolsURL, 
    currentSessionId, 
    setCurrentSession,
    isConnected,
    setConnected,
    setIntercepting,
    targets,
    setTargets,
    attachedTargets,
    toggleAttachedTarget,
    events,
    addEvent,
    clearEvents,
  } = useSessionStore()
  
  const { isDark, toggle: toggleTheme } = useThemeStore()
  const { toast } = useToast()
  const [isLoading, setIsLoading] = useState(false)
  const [isLaunchingBrowser, setIsLaunchingBrowser] = useState(false)

  // 启动浏览器
  const handleLaunchBrowser = async () => {
    setIsLaunchingBrowser(true)
    try {
      const result = await window.go?.gui?.App?.LaunchBrowser(false)
      if (result?.success) {
        setDevToolsURL(result.devToolsUrl)
        toast({
          variant: 'success',
          title: '浏览器已启动',
          description: `DevTools URL: ${result.devToolsUrl}`,
        })
      } else {
        toast({
          variant: 'destructive',
          title: '启动失败',
          description: result?.error || '无法启动浏览器',
        })
      }
    } catch (e) {
      toast({
        variant: 'destructive',
        title: '启动错误',
        description: String(e),
      })
    } finally {
      setIsLaunchingBrowser(false)
    }
  }

  // 连接/断开会话
  const handleConnect = async () => {
    if (isConnected && currentSessionId) {
      // 断开
      try {
        const result = await window.go?.gui?.App?.StopSession(currentSessionId)
        if (result?.success) {
          setConnected(false)
          setCurrentSession(null)
          setIntercepting(false)
          setTargets([])
          toast({
            variant: 'success',
            title: '已断开连接',
          })
        } else {
          toast({
            variant: 'destructive',
            title: '断开失败',
            description: result?.error,
          })
        }
      } catch (e) {
        toast({
          variant: 'destructive',
          title: '断开错误',
          description: String(e),
        })
      }
    } else {
      // 连接
      setIsLoading(true)
      try {
        const result = await window.go?.gui?.App?.StartSession(devToolsURL)
        if (result?.success) {
          setCurrentSession(result.sessionId)
          setConnected(true)
          toast({
            variant: 'success',
            title: '连接成功',
            description: `会话 ID: ${result.sessionId.slice(0, 8)}...`,
          })
          // 自动获取目标列表
          await refreshTargets(result.sessionId)
        } else {
          toast({
            variant: 'destructive',
            title: '连接失败',
            description: result?.error,
          })
        }
      } catch (e) {
        toast({
          variant: 'destructive',
          title: '连接错误',
          description: String(e),
        })
      } finally {
        setIsLoading(false)
      }
    }
  }

  // 刷新目标列表
  const refreshTargets = async (sessionId?: string) => {
    const sid = sessionId || currentSessionId
    if (!sid) return
    
    try {
      const result = await window.go?.gui?.App?.ListTargets(sid)
      if (result?.success) {
        setTargets(result.targets || [])
      }
    } catch (e) {
      console.error('List targets error:', e)
    }
  }

  // 附加/移除目标
  const handleToggleTarget = async (targetId: string) => {
    if (!currentSessionId) return
    
    const isAttached = attachedTargets.has(targetId)
    try {
      if (isAttached) {
        const result = await window.go?.gui?.App?.DetachTarget(currentSessionId, targetId)
        if (result?.success) {
          toggleAttachedTarget(targetId)
          toast({
            variant: 'success',
            title: '已移除目标',
          })
        } else {
          toast({
            variant: 'destructive',
            title: '移除失败',
            description: result?.error,
          })
        }
      } else {
        const result = await window.go?.gui?.App?.AttachTarget(currentSessionId, targetId)
        if (result?.success) {
          toggleAttachedTarget(targetId)
          toast({
            variant: 'success',
            title: '已附加目标',
          })
        } else {
          toast({
            variant: 'destructive',
            title: '附加失败',
            description: result?.error,
          })
        }
      }
    } catch (e) {
      toast({
        variant: 'destructive',
        title: '操作错误',
        description: String(e),
      })
    }
  }

  // 监听 Wails 事件
  useEffect(() => {
    // @ts-ignore
    if (window.runtime?.EventsOn) {
      // @ts-ignore
      window.runtime.EventsOn('intercept-event', (event: InterceptEvent) => {
        // 后端已提供完整事件数据，生成 id 用于前端 key
        const enrichedEvent: InterceptEvent = {
          ...event,
          id: event.id || `${event.timestamp}_${Math.random().toString(36).slice(2)}`,
        }
        addEvent(enrichedEvent)
      })
    }
  }, [addEvent])

  return (
    <div className="h-screen flex flex-col bg-background text-foreground">
      {/* 顶部工具栏 */}
      <div className="h-14 border-b flex items-center px-4 gap-4 shrink-0">
        <div className="flex items-center gap-2 flex-1">
          <Button
            onClick={handleLaunchBrowser}
            variant="outline"
            disabled={isLaunchingBrowser || isConnected}
            title="启动新浏览器实例"
          >
            <Chrome className="w-4 h-4 mr-2" />
            {isLaunchingBrowser ? '启动中...' : '启动浏览器'}
          </Button>
          <Input
            value={devToolsURL}
            onChange={(e) => setDevToolsURL(e.target.value)}
            placeholder="DevTools URL (e.g., http://localhost:9222)"
            className="w-80"
            disabled={isConnected}
          />
          <Button 
            onClick={handleConnect}
            variant={isConnected ? "destructive" : "default"}
            disabled={isLoading}
          >
            {isConnected ? <Link2Off className="w-4 h-4 mr-2" /> : <Link2 className="w-4 h-4 mr-2" />}
            {isLoading ? '连接中...' : isConnected ? '断开' : '连接'}
          </Button>
        </div>
        
        <div className="flex items-center gap-2">
          <Button 
            variant="outline" 
            size="icon"
            onClick={() => refreshTargets()}
            disabled={!isConnected}
            title="刷新目标列表"
          >
            <RefreshCw className="w-4 h-4" />
          </Button>
          {/* 状态指示器 */}
          <div className="flex items-center gap-2 text-sm">
            <span className={`flex items-center gap-1 ${isConnected ? 'text-green-500' : 'text-muted-foreground'}`}>
              <span className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-muted-foreground'}`} />
              {isConnected ? '已连接' : '未连接'}
            </span>
            {isConnected && (
              <span className="text-muted-foreground">
                · 目标 {attachedTargets.size}/{targets.length}
              </span>
            )}
          </div>
          <Button variant="ghost" size="icon" onClick={toggleTheme}>
            {isDark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </Button>
        </div>
      </div>

      {/* 主内容区 */}
      <div className="flex-1 flex flex-col overflow-hidden min-h-0">
        <Tabs defaultValue="targets" className="flex-1 flex flex-col min-h-0">
          <div className="border-b px-4">
            <TabsList className="h-10">
              <TabsTrigger value="targets" className="gap-2">
                <Link2 className="w-4 h-4" />
                Targets
              </TabsTrigger>
              <TabsTrigger value="rules" className="gap-2">
                <FileJson className="w-4 h-4" />
                Rules
              </TabsTrigger>
              <TabsTrigger value="events" className="gap-2">
                <Activity className="w-4 h-4" />
                Events
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="targets" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <div className="h-full overflow-auto p-4">
              <TargetsPanel 
                targets={targets}
                attachedTargets={attachedTargets}
                onToggle={handleToggleTarget}
                isConnected={isConnected}
              />
            </div>
          </TabsContent>

          <TabsContent value="rules" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <RulesPanel 
              sessionId={currentSessionId}
              isConnected={isConnected}
              attachedTargets={attachedTargets}
              setIntercepting={setIntercepting}
            />
          </TabsContent>

          <TabsContent value="events" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <div className="h-full overflow-auto p-4">
              <EventsPanel events={events as InterceptEvent[]} onClear={clearEvents} />
            </div>
          </TabsContent>
        </Tabs>
      </div>

      {/* 底部状态栏 */}
      <div className="h-6 border-t px-4 flex items-center text-xs text-muted-foreground shrink-0">
        <span>cdpnetool v1.0.0</span>
        <span className="mx-2">|</span>
        <span>Session: {currentSessionId?.slice(0, 8) || '-'}</span>
      </div>
      
      {/* Toast 通知 */}
      <Toaster />
    </div>
  )
}

// Targets 面板组件
function TargetsPanel({ 
  targets, 
  attachedTargets, 
  onToggle,
  isConnected 
}: { 
  targets: any[]
  attachedTargets: Set<string>
  onToggle: (id: string) => void
  isConnected: boolean
}) {
  if (!isConnected) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        请先连接到浏览器
      </div>
    )
  }

  if (targets.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        没有找到页面目标，点击刷新按钮重试
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {targets.map((target) => (
        <div 
          key={target.id}
          className="flex items-center gap-3 p-3 rounded-lg border hover:bg-muted/50 transition-colors"
        >
          <div className="flex-1 min-w-0">
            <div className="font-medium truncate">{target.title || '(无标题)'}</div>
            <div className="text-sm text-muted-foreground truncate">{target.url}</div>
          </div>
          <Button
            variant={attachedTargets.has(target.id) ? "default" : "outline"}
            size="sm"
            onClick={() => onToggle(target.id)}
          >
            {attachedTargets.has(target.id) ? '已附加' : '附加'}
          </Button>
        </div>
      ))}
    </div>
  )
}

// Rules 面板组件（可视化编辑器 + 规则集管理）
interface RulesPanelProps {
  sessionId: string | null
  isConnected: boolean
  attachedTargets: Set<string>
  setIntercepting: (intercepting: boolean) => void
}

function RulesPanel({ sessionId, isConnected, attachedTargets, setIntercepting }: RulesPanelProps) {
  const { toast } = useToast()
  const [ruleSet, setRuleSet] = useState<RuleSet>(createEmptyRuleSet())
  const [showJson, setShowJson] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  
  // 配置管理状态
  const [ruleSets, setRuleSets] = useState<RuleSetRecord[]>([])
  const [currentRuleSetId, setCurrentRuleSetId] = useState<number>(0)
  const [currentRuleSetName, setCurrentRuleSetName] = useState<string>('默认配置')
  const [currentDescription, setCurrentDescription] = useState<string>('')
  const [activeConfigId, setActiveConfigId] = useState<number | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [editingName, setEditingName] = useState<number | null>(null)
  const [newName, setNewName] = useState('')
  const [isInitializing, setIsInitializing] = useState(true)
  const [isDirty, setIsDirty] = useState(false)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; configId: number } | null>(null)

  // 组件挂载时加载配置列表
  useEffect(() => {
    loadRuleSets()
      .catch(e => {
        console.error('Failed to load rule sets on mount:', e)
        setRuleSet(createEmptyRuleSet())
      })
      .finally(() => {
        setIsInitializing(false)
      })
  }, [])

  // 点击其他地方关闭右键菜单
  useEffect(() => {
    const handleClick = () => setContextMenu(null)
    document.addEventListener('click', handleClick)
    return () => document.removeEventListener('click', handleClick)
  }, [])

  // 加载配置列表
  const loadRuleSets = async () => {
    try {
      if (!window.go?.gui?.App?.ListRuleSets) {
        console.warn('Wails bindings not ready yet')
        return
      }
      
      const result = await window.go.gui.App.ListRuleSets()
      if (result?.success) {
        setRuleSets(result.ruleSets || [])
        // 加载第一个配置到编辑器，但不自动设置为激活状态
        // 用户需要手动启用配置
        if (result.ruleSets && result.ruleSets.length > 0) {
          loadRuleSetData(result.ruleSets[0])
        } else {
          setRuleSet(createEmptyRuleSet())
        }
      }
    } catch (e) {
      console.error('Load rule sets error:', e)
      setRuleSet(createEmptyRuleSet())
    }
  }

  // 更新 Dirty 状态并通知后端
  const updateDirty = (dirty: boolean) => {
    setIsDirty(dirty)
    window.go?.gui?.App?.SetDirty(dirty)
  }

  // 处理规则变更
  const handleRulesChange = (rules: Rule[]) => {
    setRuleSet({ ...ruleSet, rules })
    updateDirty(true)
  }

  // 快捷键支持
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault()
        handleSave()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [ruleSet, currentRuleSetId, currentRuleSetName, sessionId, isLoading])

  // 加载规则集数据到编辑器
  const loadRuleSetData = (record: RuleSetRecord) => {
    try {
      if (!record.rulesJson) {
        setRuleSet(createEmptyRuleSet())
        setCurrentRuleSetId(record.id)
        setCurrentRuleSetName(record.name)
        setCurrentDescription(record.description || '')
        updateDirty(false)
        return
      }
      
      const parsed = JSON.parse(record.rulesJson)
      // 兼容两种格式：数组或 { version, rules } 对象
      if (Array.isArray(parsed)) {
        setRuleSet({ version: record.version || '1.0', rules: parsed })
      } else if (parsed.rules && Array.isArray(parsed.rules)) {
        setRuleSet({ version: parsed.version || '1.0', rules: parsed.rules })
      } else {
        console.error('Invalid rules format:', parsed)
        setRuleSet(createEmptyRuleSet())
      }
      
      setCurrentRuleSetId(record.id)
      setCurrentRuleSetName(record.name)
      setCurrentDescription(record.description || '')
      updateDirty(false)
    } catch (e) {
      console.error('Parse rules error:', e)
      setRuleSet(createEmptyRuleSet())
      updateDirty(false)
    }
  }

  // 选择规则集
  const handleSelectRuleSet = async (record: RuleSetRecord) => {
    if (isDirty) {
      const confirm = window.confirm('当前规则有未保存的更改，切换规则集将丢失这些更改，是否继续？')
      if (!confirm) return
    }
    loadRuleSetData(record)
    // 设置为激活
    await window.go?.gui?.App?.SetActiveRuleSet(record.id)
    toast({ variant: 'success', title: `已切换到规则集: ${record.name}` })
  }

  // 创建新规则集
  const handleCreateRuleSet = async () => {
    const name = `规则集 ${new Date().toLocaleString()}`
    try {
      const emptyRuleSet = { version: '1.0', rules: [] }
      const result = await window.go?.gui?.App?.SaveRuleSet(0, name, '', JSON.stringify(emptyRuleSet))
      if (result?.success && result.ruleSet) {
        await loadRuleSets()
        loadRuleSetData(result.ruleSet)
        await window.go?.gui?.App?.SetActiveRuleSet(result.ruleSet.id)
        toast({ variant: 'success', title: '新规则集已创建' })
      }
    } catch (e) {
      toast({ variant: 'destructive', title: '创建失败', description: String(e) })
    }
  }

  // 删除规则集
  const handleDeleteRuleSet = async (id: number) => {
    if (ruleSets.length <= 1) {
      toast({ variant: 'destructive', title: '至少保留一个规则集' })
      return
    }
    try {
      const result = await window.go?.gui?.App?.DeleteRuleSet(id)
      if (result?.success) {
        await loadRuleSets()
        // 如果删除的是当前规则集，切换到第一个
        if (id === currentRuleSetId) {
          const remaining = ruleSets.filter(r => r.id !== id)
          if (remaining.length > 0) {
            loadRuleSetData(remaining[0])
            await window.go?.gui?.App?.SetActiveRuleSet(remaining[0].id)
          }
        }
        toast({ variant: 'success', title: '规则集已删除' })
      }
    } catch (e) {
      toast({ variant: 'destructive', title: '删除失败', description: String(e) })
    }
  }

  // 重命名规则集
  const handleRenameRuleSet = async (id: number) => {
    if (!newName.trim()) return
    try {
      const result = await window.go?.gui?.App?.RenameRuleSet(id, newName.trim())
      if (result?.success) {
        await loadRuleSets()
        if (id === currentRuleSetId) {
          setCurrentRuleSetName(newName.trim())
        }
        setEditingName(null)
        setNewName('')
        toast({ variant: 'success', title: '已重命名' })
      }
    } catch (e) {
      toast({ variant: 'destructive', title: '重命名失败', description: String(e) })
    }
  }

  // 启用/禁用配置
  const handleToggleConfig = async (config: RuleSetRecord, enabled: boolean) => {
    if (enabled) {
      // 启用配置
      if (!isConnected) {
        toast({ variant: 'destructive', title: '请先连接到浏览器' })
        return
      }
      if (attachedTargets.size === 0) {
        toast({ variant: 'destructive', title: '请先在 Targets 标签页附加至少一个目标' })
        return
      }
      
      try {
        // 加载规则到会话 - 需要确保是完整的 Config 格式
        let configJson: string
        if (config.rulesJson) {
          const parsed = JSON.parse(config.rulesJson)
          if (Array.isArray(parsed)) {
            // 如果是数组格式，包装成完整的 Config
            configJson = JSON.stringify({ version: config.version || '1.0', rules: parsed })
          } else if (parsed.version && parsed.rules) {
            // 已经是完整格式
            configJson = config.rulesJson
          } else {
            configJson = JSON.stringify({ version: '1.0', rules: [] })
          }
        } else {
          configJson = JSON.stringify({ version: '1.0', rules: [] })
        }
        const loadResult = await window.go?.gui?.App?.LoadRules(sessionId!, configJson)
        if (!loadResult?.success) {
          toast({ variant: 'destructive', title: '加载规则失败', description: loadResult?.error })
          return
        }
        
        // 启用拦截
        const enableResult = await window.go?.gui?.App?.EnableInterception(sessionId!)
        if (!enableResult?.success) {
          toast({ variant: 'destructive', title: '启用拦截失败', description: enableResult?.error })
          return
        }
        
        // 设置为激活状态
        await window.go?.gui?.App?.SetActiveRuleSet(config.id)
        setActiveConfigId(config.id)
        setIntercepting(true)
        await loadRuleSets()
        
        toast({ variant: 'success', title: `配置「${config.name}」已启用` })
      } catch (e) {
        toast({ variant: 'destructive', title: '启用失败', description: String(e) })
      }
    } else {
      // 禁用配置
      try {
        if (sessionId) {
          await window.go?.gui?.App?.DisableInterception(sessionId)
        }
        setActiveConfigId(null)
        setIntercepting(false)
        toast({ variant: 'success', title: '拦截已停止' })
      } catch (e) {
        toast({ variant: 'destructive', title: '停止失败', description: String(e) })
      }
    }
  }

  // 计算配置中的规则数量
  const getRuleCount = (config: RuleSetRecord) => {
    try {
      const parsed = JSON.parse(config.rulesJson || '[]')
      return Array.isArray(parsed) ? parsed.length : (parsed.rules?.length || 0)
    } catch {
      return 0
    }
  }

  // 右键菜单
  const handleContextMenu = (e: React.MouseEvent, configId: number) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, configId })
  }

  // 添加新规则
  const handleAddRule = () => {
    setRuleSet({
      ...ruleSet,
      rules: [...ruleSet.rules, createEmptyRule()]
    })
    updateDirty(true)
  }

  // 保存配置
  const handleSave = async () => {
    setIsLoading(true)
    try {
      const rulesJson = JSON.stringify(ruleSet)
      
      const saveResult = await window.go?.gui?.App?.SaveRuleSet(
        currentRuleSetId,
        currentRuleSetName,
        currentDescription,
        rulesJson
      )
      
      if (!saveResult?.success) {
        toast({ variant: 'destructive', title: '保存失败', description: saveResult?.error })
        return
      }
      
      if (saveResult.ruleSet) {
        setCurrentRuleSetId(saveResult.ruleSet.id)
      }
      
      updateDirty(false)
      await loadRuleSets()
      
      // 如果当前配置是激活状态，重新加载规则到会话
      if (currentRuleSetId === activeConfigId && sessionId) {
        await window.go?.gui?.App?.LoadRules(sessionId, rulesJson)
        toast({ variant: 'success', title: `已保存并更新 ${ruleSet.rules.length} 条规则` })
      } else {
        toast({ variant: 'success', title: `已保存 ${ruleSet.rules.length} 条规则` })
      }
    } catch (e) {
      toast({ variant: 'destructive', title: '保存失败', description: String(e) })
    } finally {
      setIsLoading(false)
    }
  }

  // 导出 JSON (原生对话框)
  const handleExport = async () => {
    const json = JSON.stringify(ruleSet, null, 2)
    const result = await window.go?.gui?.App?.ExportRuleSet(currentRuleSetName || "ruleset", json)
    if (result && !result.success) {
      toast({ variant: 'destructive', title: '导出失败', description: result.error })
    } else if (result && result.success) {
      toast({ variant: 'success', title: '规则集导出成功' })
    }
  }

  // 导入 JSON
  const handleImport = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    
    const reader = new FileReader()
    reader.onload = (event) => {
      try {
        const json = event.target?.result as string
        const imported = JSON.parse(json) as RuleSet
        if (imported.version && Array.isArray(imported.rules)) {
          setRuleSet(imported)
          updateDirty(true)
          toast({ variant: 'success', title: `导入成功，共 ${imported.rules.length} 条规则（请点保存以持久化）` })
        } else {
          toast({ variant: 'destructive', title: 'JSON 格式不正确' })
        }
      } catch {
        toast({ variant: 'destructive', title: 'JSON 解析失败' })
      }
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  return (
    <div className="flex-1 flex min-h-0">
      {/* 初始化加载状态 */}
      {isInitializing ? (
        <div className="flex items-center justify-center w-full text-muted-foreground">
          <div className="text-center">
            <div className="text-lg mb-2">加载中...</div>
            <div className="text-sm">正在初始化配置编辑器</div>
          </div>
        </div>
      ) : (
        <>
          {/* 左侧配置列表 */}
          <div className="w-60 border-r flex flex-col shrink-0">
            <div className="p-3 border-b flex items-center justify-between">
              <span className="font-medium text-sm">配置列表</span>
              <Button size="sm" variant="ghost" onClick={handleCreateRuleSet} title="新建配置">
                <Plus className="w-4 h-4" />
              </Button>
            </div>
            <ScrollArea className="flex-1">
              <div className="p-2 space-y-1">
                {ruleSets.map((config) => (
                  <div
                    key={config.id}
                    className={`flex items-center gap-2 p-2 rounded-md cursor-pointer transition-colors ${
                      config.id === currentRuleSetId 
                        ? 'bg-primary/10 border border-primary/30' 
                        : 'hover:bg-muted'
                    }`}
                    onClick={() => handleSelectRuleSet(config)}
                    onContextMenu={(e) => handleContextMenu(e, config.id)}
                  >
                    <Switch
                      checked={config.id === activeConfigId}
                      onCheckedChange={(checked) => handleToggleConfig(config, checked)}
                      disabled={!isConnected && config.id !== activeConfigId}
                    />
                    <div className="flex-1 min-w-0">
                      {editingName === config.id ? (
                        <Input
                          value={newName}
                          onChange={(e) => setNewName(e.target.value)}
                          className="h-6 text-sm"
                          autoFocus
                          onClick={(e) => e.stopPropagation()}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') handleRenameRuleSet(config.id)
                            if (e.key === 'Escape') { setEditingName(null); setNewName('') }
                          }}
                          onBlur={() => { setEditingName(null); setNewName('') }}
                        />
                      ) : (
                        <>
                          <div className="text-sm font-medium truncate">{config.name}</div>
                          <div className="text-xs text-muted-foreground">
                            {getRuleCount(config)} 条规则
                            {config.id === activeConfigId && <span className="ml-1 text-green-500">· 运行中</span>}
                          </div>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </ScrollArea>
          </div>

          {/* 右键菜单 */}
          {contextMenu && (
            <div
              className="fixed z-50 min-w-32 bg-popover border rounded-md shadow-md p-1"
              style={{ left: contextMenu.x, top: contextMenu.y }}
            >
              <button
                className="w-full text-left px-2 py-1.5 text-sm rounded hover:bg-muted flex items-center gap-2"
                onClick={() => {
                  const config = ruleSets.find(r => r.id === contextMenu.configId)
                  if (config) {
                    setEditingName(config.id)
                    setNewName(config.name)
                  }
                  setContextMenu(null)
                }}
              >
                <Edit3 className="w-4 h-4" />
                重命名
              </button>
              <button
                className="w-full text-left px-2 py-1.5 text-sm rounded hover:bg-muted flex items-center gap-2 text-destructive disabled:opacity-50"
                onClick={() => {
                  handleDeleteRuleSet(contextMenu.configId)
                  setContextMenu(null)
                }}
                disabled={ruleSets.length <= 1}
              >
                <Trash2 className="w-4 h-4" />
                删除
              </button>
            </div>
          )}

          {/* 右侧配置详情 */}
          <div className="flex-1 flex flex-col min-h-0 p-4">
            {/* 配置信息栏 */}
            <div className="flex flex-wrap items-center gap-4 mb-4 pb-3 border-b shrink-0">
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground whitespace-nowrap">配置名称:</span>
                <Input
                  value={currentRuleSetName}
                  onChange={(e) => {
                    setCurrentRuleSetName(e.target.value)
                    updateDirty(true)
                  }}
                  className="w-40 h-8"
                />
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground whitespace-nowrap">描述:</span>
                <Input
                  value={currentDescription}
                  onChange={(e) => {
                    setCurrentDescription(e.target.value)
                    updateDirty(true)
                  }}
                  placeholder="配置描述（可选）"
                  className="w-48 h-8"
                />
                {isDirty && <span className="w-2 h-2 rounded-full bg-primary animate-pulse" title="有未保存更改" />}
              </div>
              <div className="flex-1" />
              <input
                ref={fileInputRef}
                type="file"
                accept=".json"
                onChange={handleImport}
                className="hidden"
              />
              <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>
                <Upload className="w-4 h-4 mr-1" />
                导入
              </Button>
              <Button variant="outline" size="sm" onClick={handleExport}>
                <Download className="w-4 h-4 mr-1" />
                导出
              </Button>
              <Button size="sm" onClick={handleSave} disabled={isLoading}>
                <Save className="w-4 h-4 mr-1" />
                {isLoading ? '保存中...' : '保存'}
              </Button>
            </div>

            {/* 规则工具栏 */}
            <div className="flex items-center gap-2 mb-4 shrink-0">
              <Button onClick={handleAddRule} size="sm">
                <Plus className="w-4 h-4 mr-1" />
                添加规则
              </Button>
              <Button variant="outline" size="sm" onClick={() => setShowJson(!showJson)}>
                <FileJson className="w-4 h-4 mr-1" />
                {showJson ? '可视化' : 'JSON'}
              </Button>
              <div className="flex-1" />
              <span className="text-xs text-muted-foreground">
                共 {ruleSet.rules.length} 条规则
              </span>
            </div>

            {/* 规则编辑区 */}
            <div className="flex-1 min-h-0 overflow-auto">
              {showJson ? (
                <textarea
                  value={JSON.stringify(ruleSet, null, 2)}
                  onChange={(e) => {
                    try {
                      setRuleSet(JSON.parse(e.target.value))
                      updateDirty(true)
                    } catch {}
                  }}
                  className="w-full h-full p-3 rounded-md border bg-background font-mono text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring"
                />
              ) : (
                <RuleListEditor
                  rules={ruleSet.rules}
                  onChange={handleRulesChange}
                />
              )}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

export default App
