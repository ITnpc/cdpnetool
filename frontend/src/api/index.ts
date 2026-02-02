import * as App from '../../wailsjs/go/gui/App'

/**
 * 核心 API 服务层
 * 封装 Wails 自动生成的 Go 绑定，提供统一的 Promise 接口和类型支持
 */
export const api = {
  // 会话管理
  session: {
    start: App.StartSession,
    stop: App.StopSession,
    getCurrent: App.GetCurrentSession,
    enableInterception: App.EnableInterception,
    disableInterception: App.DisableInterception,
    loadRules: App.LoadRules,
    getRuleStats: App.GetRuleStats,
    enableTrafficCapture: App.EnableTrafficCapture,
    loadActiveConfig: App.LoadActiveConfigToSession,
  },
  
  // 浏览器控制
  browser: {
    launch: App.LaunchBrowser,
    close: App.CloseBrowser,
    getStatus: App.GetBrowserStatus,
    listTargets: App.ListTargets,
    attachTarget: App.AttachTarget,
    detachTarget: App.DetachTarget,
  },
  
  // 配置管理
  config: {
    list: App.ListConfigs,
    get: App.GetConfig,
    save: App.SaveConfig,
    delete: App.DeleteConfig,
    setActive: App.SetActiveConfig,
    getActive: App.GetActiveConfig,
    rename: App.RenameConfig,
    import: App.ImportConfig,
    export: App.ExportConfig,
    create: App.CreateNewConfig,
    generateRule: App.GenerateNewRule,
    setDirty: App.SetDirty,
  },
  
  // 设置与系统
  system: {
    getVersion: App.GetVersion,
    getSettings: App.GetAllSettings,
    getSetting: App.GetSetting,
    setSetting: App.SetSetting,
    setMultipleSettings: App.SetMultipleSettings,
  },
  
  // 历史记录
  history: {
    queryEvents: App.QueryMatchedEventHistory,
    cleanupEvents: App.CleanupEventHistory,
  }
}
