import { create } from 'zustand'
import type { InterceptEvent } from '@/types/events'

// 类型定义
export interface TargetInfo {
  id: string
  type: string
  url: string
  title: string
  isCurrent: boolean
  isUser: boolean
}

export interface EngineStats {
  total: number
  matched: number
  byRule: Record<string, number>
}

// Session 状态
interface SessionState {
  currentSessionId: string | null
  devToolsURL: string
  isConnected: boolean
  isIntercepting: boolean
  targets: TargetInfo[]
  attachedTargets: Set<string>
  events: InterceptEvent[]
  stats: EngineStats | null
  
  // Actions
  setDevToolsURL: (url: string) => void
  setCurrentSession: (id: string | null) => void
  setConnected: (connected: boolean) => void
  setIntercepting: (intercepting: boolean) => void
  setTargets: (targets: TargetInfo[]) => void
  toggleAttachedTarget: (targetId: string) => void
  addEvent: (event: InterceptEvent) => void
  clearEvents: () => void
  setStats: (stats: EngineStats | null) => void
}

export const useSessionStore = create<SessionState>((set) => ({
  currentSessionId: null,
  devToolsURL: 'http://localhost:9222',
  isConnected: false,
  isIntercepting: false,
  targets: [],
  attachedTargets: new Set(),
  events: [],
  stats: null,
  
  setDevToolsURL: (url) => set({ devToolsURL: url }),
  setCurrentSession: (id) => set({ currentSessionId: id }),
  setConnected: (connected) => set({ isConnected: connected }),
  setIntercepting: (intercepting) => set({ isIntercepting: intercepting }),
  setTargets: (targets) => set({ targets }),
  toggleAttachedTarget: (targetId) => set((state) => {
    const newSet = new Set(state.attachedTargets)
    if (newSet.has(targetId)) {
      newSet.delete(targetId)
    } else {
      newSet.add(targetId)
    }
    return { attachedTargets: newSet }
  }),
  addEvent: (event) => set((state) => ({
    events: [event, ...state.events].slice(0, 100) // 只保留最新 100 条
  })),
  clearEvents: () => set({ events: [] }),
  setStats: (stats) => set({ stats }),
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
