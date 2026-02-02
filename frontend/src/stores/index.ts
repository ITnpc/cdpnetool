import { create } from 'zustand'
import type { 
  NetworkEvent,
  MatchedEventWithId 
} from '@/types/events'
import { api } from '@/api'
import { domain } from '../../wailsjs/go/models'

// Session 状态
interface SessionState {
  currentSessionId: string | null
  devToolsURL: string
  isConnected: boolean
  isIntercepting: boolean
  activeConfigId: number | null
  targets: domain.TargetInfo[]
  attachedTargetId: string | null
  matchedEvents: MatchedEventWithId[]    // 匹配的事件（会存入数据库）
  isTrafficCapturing: boolean           // 是否正在捕获全量流量
  trafficEvents: NetworkEvent[]         // 全量流量列表（仅内存，最近100条）
  
  // Actions
  setDevToolsURL: (url: string) => void
  setCurrentSession: (id: string | null) => void
  setConnected: (connected: boolean) => void
  setIntercepting: (intercepting: boolean) => void
  setTrafficCapturing: (capturing: boolean) => void
  setActiveConfigId: (id: number | null) => void
  setTargets: (targets: domain.TargetInfo[]) => void
  setAttachedTargetId: (targetId: string | null) => void
  resetSession: () => void
  
  // 复杂业务 Actions
  refreshTargets: () => Promise<void>
  toggleTarget: (targetId: string) => Promise<{ success: boolean; message?: string }>
  
  // 事件操作
  addInterceptEvent: (event: NetworkEvent) => void
  addTrafficEvent: (event: NetworkEvent) => void
  clearMatchedEvents: () => void
  clearTrafficEvents: () => void
  clearAllEvents: () => void
}

// 生成事件 ID
function generateEventId(timestamp: number): string {
  return `${timestamp}_${Math.random().toString(36).slice(2, 10)}`
}

export const useSessionStore = create<SessionState>((set, get) => ({
  currentSessionId: null,
  devToolsURL: 'http://localhost:9222',
  isConnected: false,
  isIntercepting: false,
  activeConfigId: null,
  targets: [],
  attachedTargetId: null,
  matchedEvents: [],
  isTrafficCapturing: false,
  trafficEvents: [],
  
  setDevToolsURL: (url) => set({ devToolsURL: url }),
  setCurrentSession: (id) => set({ currentSessionId: id }),
  setConnected: (connected) => set({ isConnected: connected }),
  setIntercepting: (intercepting) => set({ isIntercepting: intercepting }),
  setTrafficCapturing: (capturing) => set({ isTrafficCapturing: capturing }),
  setActiveConfigId: (id) => set({ activeConfigId: id }),
  setTargets: (targets) => set({ targets }),
  setAttachedTargetId: (targetId) => set({ attachedTargetId: targetId }),

  resetSession: () => set({
    attachedTargetId: null,
    activeConfigId: null,
    isIntercepting: false,
    isTrafficCapturing: false,
    targets: [],
    trafficEvents: [],
  }),

  // 刷新目标列表
  refreshTargets: async () => {
    const { currentSessionId: sessionId } = get()
    if (!sessionId) return
    
    try {
      const result = await api.browser.listTargets(sessionId)
      if (result?.success && result.data) {
        set({ targets: result.data.targets || [] })
      }
    } catch (e) {
      console.error('List targets error:', e)
    }
  },

  // 切换目标
  toggleTarget: async (targetId: string) => {
    const { 
      currentSessionId: sessionId, 
      attachedTargetId, 
      isIntercepting, 
      isTrafficCapturing 
    } = get()
    
    if (!sessionId) return { success: false, message: '会话未启动' }
    
    const isCurrentlyAttached = attachedTargetId === targetId
    
    try {
      // 1. 如果正在拦截或捕获，切换任何目标状态前必须先停止
      if (isIntercepting || isTrafficCapturing) {
        if (isIntercepting) {
          await api.session.disableInterception(sessionId)
          set({ isIntercepting: false, activeConfigId: null })
        }
        if (isTrafficCapturing) {
          await api.session.enableTrafficCapture(sessionId, false)
          set({ isTrafficCapturing: false })
        }
      }

      // 2. 如果点击的是当前已附着的目标 -> 执行“分离”
      if (isCurrentlyAttached) {
        const result = await api.browser.detachTarget(sessionId, targetId)
        if (result?.success) {
          set({ attachedTargetId: null })
          return { success: true }
        }
        return { success: false, message: result?.message }
      }

      // 3. 如果点击的是新目标 -> 先分离旧的（如果有），再附着新的
      if (attachedTargetId) {
        await api.browser.detachTarget(sessionId, attachedTargetId)
      }

      const result = await api.browser.attachTarget(sessionId, targetId)
      if (result?.success) {
        set({ attachedTargetId: targetId })
        return { success: true }
      }
      
      set({ attachedTargetId: null })
      return { success: false, message: result?.message }
    } catch (e) {
      return { success: false, message: String(e) }
    }
  },
  
  // 添加事件
  addInterceptEvent: (event) => set((state) => {
    console.log('[Store] 处理拦截事件:', event)
    
    if (event.isMatched) {
      const eventWithId: MatchedEventWithId = {
        networkEvent: event,
        id: generateEventId(event.timestamp),
      }
      return {
        matchedEvents: [eventWithId, ...state.matchedEvents].slice(0, 200)
      }
    }
    return state
  }),

  // 添加全量流量事件 (Upsert 逻辑)
  addTrafficEvent: (event) => set((state) => {
    const existingIndex = state.trafficEvents.findIndex(e => e.id === event.id)
    if (existingIndex > -1) {
      const newList = [...state.trafficEvents]
      newList[existingIndex] = { ...newList[existingIndex], ...event }
      return { trafficEvents: newList }
    } else {
      return { trafficEvents: [event, ...state.trafficEvents].slice(0, 100) }
    }
  }),
  
  clearMatchedEvents: () => set({ matchedEvents: [] }),
  clearTrafficEvents: () => set({ trafficEvents: [] }),
  clearAllEvents: () => set({ matchedEvents: [], trafficEvents: [] }),
}))

// 主题状态
interface ThemeState {
  isDark: boolean
  toggle: () => void
}

export const useThemeStore = create<ThemeState>((set) => ({
  isDark: true,
  toggle: () => set((state) => {
    const newIsDark = !state.isDark
    if (newIsDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    return { isDark: newIsDark }
  }),
}))

// 初始化主题
if (typeof window !== 'undefined') {
  document.documentElement.classList.add('dark')
}
