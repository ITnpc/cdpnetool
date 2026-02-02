import { useState, useEffect } from 'react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Toaster } from '@/components/ui/toaster'
import { useToast } from '@/hooks/use-toast'
import { useSessionStore, useThemeStore } from '@/stores'
import { EventsPanel } from '@/components/events'
import { NetworkPanel } from '@/components/network/NetworkPanel'
import { TargetsPanel } from '@/components/targets/TargetsPanel'
import { RulesPanel } from '@/components/rules/RulesPanel'
import { api } from '@/api'
import { 
  RefreshCw, 
  Moon, 
  Sun,
  Link2,
  Link2Off,
  FileJson,
  Activity,
  Chrome,
} from 'lucide-react'

function App() {
  const { 
    devToolsURL, 
    setDevToolsURL, 
    currentSessionId: sessionId, 
    setCurrentSession,
    isConnected,
    setConnected,
    setIntercepting,
    targets,
    attachedTargetId,
    matchedEvents,
    trafficEvents,
    isTrafficCapturing,
    setTrafficCapturing,
    addInterceptEvent,
    addTrafficEvent,
    clearMatchedEvents,
    clearTrafficEvents,
    resetSession,
    refreshTargets,
    toggleTarget,
  } = useSessionStore()
  
  const { isDark, toggle: toggleTheme } = useThemeStore()
  const { toast } = useToast()
  const [isLoading, setIsLoading] = useState(false)
  const [isLaunchingBrowser, setIsLaunchingBrowser] = useState(false)
  const [appVersion, setAppVersion] = useState('')

  // 获取版本号
  useEffect(() => {
    const fetchVersion = async () => {
      try {
        const result = await api.system.getVersion()
        if (result?.success && result.data) {
          setAppVersion(result.data.version)
        }
      } catch (e) {
        console.error('获取版本号失败:', e)
      }
    }
    fetchVersion()
  }, [])

  // 启动浏览器
  const handleLaunchBrowser = async () => {
    setIsLaunchingBrowser(true)
    try {
      const result = await api.browser.launch(false)
      if (result?.success && result.data) {
        setDevToolsURL(result.data.devToolsUrl)
        toast({
          variant: 'success',
          title: '浏览器已启动',
          description: `DevTools URL: ${result.data.devToolsUrl}`,
        })
      } else {
        toast({
          variant: 'destructive',
          title: '启动失败',
          description: result?.message || '无法启动浏览器',
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
    if (isConnected && sessionId) {
      try {
        const result = await api.session.stop(sessionId)
        if (result?.success) {
          setConnected(false)
          setCurrentSession(null)
          resetSession()
          toast({ variant: 'success', title: '已断开连接' })
        } else {
          toast({ variant: 'destructive', title: '断开失败', description: result?.message })
        }
      } catch (e) {
        toast({ variant: 'destructive', title: '断开错误', description: String(e) })
      }
    } else {
      setIsLoading(true)
      try {
        const result = await api.session.start(devToolsURL)
        if (result?.success && result.data) {
          setCurrentSession(result.data.sessionId)
          setConnected(true)
          toast({
            variant: 'success',
            title: '连接成功',
            description: `会话 ID: ${result.data.sessionId.slice(0, 8)}...`,
          })
          await refreshTargets()
        } else {
          toast({ variant: 'destructive', title: '连接失败', description: result?.message || '连接失败' })
        }
      } catch (e) {
        toast({ variant: 'destructive', title: '连接错误', description: String(e) })
      } finally {
        setIsLoading(false)
      }
    }
  }

  // 切换目标处理
  const handleToggleTarget = async (targetId: string) => {
    const result = await toggleTarget(targetId)
    if (!result.success) {
      toast({ variant: 'destructive', title: '操作失败', description: result.message })
    }
  }

  // 切换全量流量捕获
  const handleToggleTrafficCapture = async (enabled: boolean) => {
    if (!sessionId) return
    try {
      const result = await api.session.enableTrafficCapture(sessionId, enabled)
      if (result?.success) {
        setTrafficCapturing(enabled)
        toast({ 
          variant: enabled ? 'success' : 'default',
          title: enabled ? '开启捕获' : '停止捕获',
          description: enabled ? '现在将记录所有网络请求' : '已停止全量请求记录'
        })
      } else {
        toast({ variant: 'destructive', title: '操作失败', description: result?.message })
      }
    } catch (e) {
      toast({ variant: 'destructive', title: '操作错误', description: String(e) })
    }
  }

  // 监听 Wails 事件
  useEffect(() => {
    // @ts-ignore
    if (window.runtime?.EventsOn) {
      // @ts-ignore
      const unsubscribeIntercept = window.runtime.EventsOn('intercept-event', addInterceptEvent)
      // @ts-ignore
      const unsubscribeTraffic = window.runtime.EventsOn('traffic-event', addTrafficEvent)
      
      return () => {
        if (unsubscribeIntercept) unsubscribeIntercept()
        if (unsubscribeTraffic) unsubscribeTraffic()
      }
    }
  }, [addInterceptEvent, addTrafficEvent])

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
          <div className="flex items-center gap-2 text-sm">
            <span className={`flex items-center gap-1 ${isConnected ? 'text-green-500' : 'text-muted-foreground'}`}>
              <span className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-muted-foreground'}`} />
              {isConnected ? '已连接' : '未连接'}
            </span>
            {isConnected && (
              <span className="text-muted-foreground">
                · 目标 {attachedTargetId ? 1 : 0}/1
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
              <TabsTrigger value="network" className="gap-2">
                <Activity className="w-4 h-4" />
                Network
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="targets" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <div className="h-full overflow-auto p-4">
              <TargetsPanel 
                targets={targets}
                attachedTargetId={attachedTargetId}
                onToggle={handleToggleTarget}
                isConnected={isConnected}
              />
            </div>
          </TabsContent>

          <TabsContent value="rules" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <RulesPanel 
              sessionId={sessionId}
              isConnected={isConnected}
              attachedTargetId={attachedTargetId}
              setIntercepting={setIntercepting}
            />
          </TabsContent>

          <TabsContent value="events" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <div className="h-full overflow-auto p-4">
              <EventsPanel 
                matchedEvents={matchedEvents} 
                onClearMatched={clearMatchedEvents}
              />
            </div>
          </TabsContent>

          <TabsContent value="network" className="flex-1 overflow-hidden m-0 min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
            <div className="h-full overflow-auto p-4">
              <NetworkPanel 
                events={trafficEvents}
                isCapturing={isTrafficCapturing}
                onToggleCapture={handleToggleTrafficCapture}
                onClear={clearTrafficEvents}
                isConnected={isConnected}
              />
            </div>
          </TabsContent>
        </Tabs>
      </div>
      
      <div className="h-6 border-t px-4 flex items-center text-xs text-muted-foreground shrink-0">
        <span>cdpnetool v{appVersion}</span>
        <span className="mx-2">|</span>
        <span>Session: {sessionId?.slice(0, 8) || '-'}</span>
      </div>
      
      <Toaster />
    </div>
  )
}

export default App
