import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Folder, RotateCcw, Settings as SettingsIcon, Monitor } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/hooks/use-toast'
import { api } from '@/api'
import { useSessionStore, useThemeStore } from '@/stores'
import { cn } from '@/lib/utils'

interface SettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type SettingsTab = 'general' | 'browser'

export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const { setLanguage } = useSessionStore()
  const { setMode: setThemeMode } = useThemeStore()

  const [activeTab, setActiveTab] = useState<SettingsTab>('general')

  // 表单状态
  const [formData, setFormData] = useState({
    language: 'zh',
    theme: 'system',
    browser_args: '',
    browser_path: '',
  })

  const [isLoading, setIsLoading] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  // 加载设置
  useEffect(() => {
    if (open) {
      loadSettings()
    }
  }, [open])

  const loadSettings = async () => {
    setIsLoading(true)
    try {
      const result = await api.system.getSettings()
      if (result?.success && result.data?.settings) {
        const settings = result.data.settings
        setFormData({
          language: settings.language || 'zh',
          theme: settings.theme || 'system',
          browser_args: settings.browser_args || '',
          browser_path: settings.browser_path || '',
        })
      }
    } catch (e) {
      console.error('加载设置失败:', e)
      toast({
        variant: 'destructive',
        title: t('settings.loadFailed'),
        description: String(e),
      })
    } finally {
      setIsLoading(false)
    }
  }

  // 保存设置
  const handleSave = async () => {
    setIsSaving(true)
    try {
      const result = await api.system.saveSettings(formData)
      if (result?.success) {
        // 应用语言和主题设置到前端
        setLanguage(formData.language)
        setThemeMode(formData.theme as 'light' | 'dark' | 'system')
        
        toast({
          variant: 'success',
          title: t('settings.saveSuccess'),
        })
        onOpenChange(false)
      } else {
        toast({
          variant: 'destructive',
          title: t('settings.saveFailed'),
          description: result?.message,
        })
      }
    } catch (e) {
      toast({
        variant: 'destructive',
        title: t('settings.saveFailed'),
        description: String(e),
      })
    } finally {
      setIsSaving(false)
    }
  }

  // 恢复默认设置
  const handleReset = async () => {
    try {
      const result = await api.system.resetSettings()
      if (result?.success && result.data?.settings) {
        const settings = result.data.settings
        setFormData({
          language: settings.language || 'zh',
          theme: settings.theme || 'system',
          browser_args: settings.browser_args || '',
          browser_path: settings.browser_path || '',
        })
        toast({
          variant: 'success',
          title: t('settings.resetSuccess'),
        })
      }
    } catch (e) {
      toast({
        variant: 'destructive',
        title: t('settings.resetFailed'),
        description: String(e),
      })
    }
  }

  // 选择浏览器路径
  const handleSelectBrowserPath = async () => {
    try {
      const result = await api.system.selectBrowserPath()
      if (result?.success && result.data?.value) {
        setFormData({ ...formData, browser_path: result.data.value })
      }
    } catch (e) {
      // 用户取消选择，不显示错误
      if (String(e).includes('CANCELLED')) return
      
      toast({
        variant: 'destructive',
        title: t('settings.selectFileFailed'),
        description: String(e),
      })
    }
  }

  const tabs = [
    { id: 'general' as const, label: t('settings.general.title'), icon: SettingsIcon },
    { id: 'browser' as const, label: t('settings.browser.title'), icon: Monitor },
  ]

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl h-[600px] p-0 gap-0 flex flex-col">
        {/* 标题栏 */}
        <DialogHeader className="px-6 py-4 border-b shrink-0">
          <DialogTitle className="text-base">{t('settings.title')}</DialogTitle>
        </DialogHeader>

        {/* 主体区域：左侧边栏 + 右侧内容 */}
        <div className="flex flex-1 overflow-hidden min-h-0">
          {/* 左侧导航 */}
          <div className="w-48 border-r bg-muted/30 shrink-0">
            <div className="p-2 space-y-1">
              {tabs.map((tab) => {
                const Icon = tab.icon
                return (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id)}
                    className={cn(
                      "w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors",
                      activeTab === tab.id
                        ? "bg-background text-foreground shadow-sm"
                        : "text-muted-foreground hover:bg-background/50 hover:text-foreground"
                    )}
                  >
                    <Icon className="w-4 h-4" />
                    {tab.label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* 右侧内容 */}
          <div className="flex-1 overflow-y-auto">
            <div className="p-6 space-y-6">
              {isLoading ? (
                <div className="flex items-center justify-center h-full">
                  <span className="text-muted-foreground">{t('common.loading')}</span>
                </div>
              ) : (
                <>
                  {/* 通用设置 */}
                  {activeTab === 'general' && (
                    <div className="space-y-6">
                      <div>
                        <h3 className="text-sm font-medium mb-4">{t('settings.general.title')}</h3>
                        <div className="space-y-4">
                          {/* 语言 */}
                          <div className="grid grid-cols-4 gap-4 items-center">
                            <Label htmlFor="language" className="text-right text-sm">
                              {t('settings.general.language')}
                            </Label>
                            <div className="col-span-3">
                              <Select
                                id="language"
                                value={formData.language}
                                onChange={(e) => setFormData({ ...formData, language: e.target.value })}
                                options={[
                                  { value: 'zh', label: '简体中文' },
                                  { value: 'en', label: 'English' },
                                ]}
                                className="w-full"
                              />
                            </div>
                          </div>

                          {/* 主题 */}
                          <div className="grid grid-cols-4 gap-4 items-center">
                            <Label htmlFor="theme" className="text-right text-sm">
                              {t('settings.general.theme')}
                            </Label>
                            <div className="col-span-3">
                              <Select
                                id="theme"
                                value={formData.theme}
                                onChange={(e) => setFormData({ ...formData, theme: e.target.value })}
                                options={[
                                  { value: 'light', label: t('settings.general.themeLight') },
                                  { value: 'dark', label: t('settings.general.themeDark') },
                                  { value: 'system', label: t('settings.general.themeSystem') },
                                ]}
                                className="w-full"
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* 浏览器设置 */}
                  {activeTab === 'browser' && (
                    <div className="space-y-6">
                      <div>
                        <h3 className="text-sm font-medium mb-4">{t('settings.browser.title')}</h3>
                        <div className="space-y-4">
                          {/* 浏览器启动参数 */}
                          <div className="grid grid-cols-4 gap-4 items-start">
                            <Label htmlFor="browser_args" className="text-right text-sm pt-2">
                              {t('settings.browser.args')}
                            </Label>
                            <div className="col-span-3 space-y-1">
                              <Textarea
                                id="browser_args"
                                value={formData.browser_args}
                                onChange={(e) => setFormData({ ...formData, browser_args: e.target.value })}
                                placeholder="--disable-extensions --disable-gpu"
                                rows={3}
                              />
                              <p className="text-xs text-muted-foreground">
                                {t('settings.browser.argsDesc')}
                              </p>
                            </div>
                          </div>

                          {/* 浏览器可执行文件路径 */}
                          <div className="grid grid-cols-4 gap-4 items-start">
                            <Label htmlFor="browser_path" className="text-right text-sm pt-2">
                              {t('settings.browser.path')}
                            </Label>
                            <div className="col-span-3 space-y-1">
                              <div className="flex gap-2">
                                <Input
                                  id="browser_path"
                                  value={formData.browser_path}
                                  onChange={(e) => setFormData({ ...formData, browser_path: e.target.value })}
                                  placeholder={t('settings.browser.pathPlaceholder')}
                                  className="flex-1"
                                />
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="icon"
                                  onClick={handleSelectBrowserPath}
                                  title={t('settings.browser.selectFile')}
                                >
                                  <Folder className="w-4 h-4" />
                                </Button>
                              </div>
                              <p className="text-xs text-muted-foreground">
                                {t('settings.browser.pathDesc')}
                              </p>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                </>
              )}
            </div>
          </div>
        </div>

        {/* 底部操作栏 */}
        <div className="flex justify-between items-center px-6 py-3 border-t bg-muted/30 shrink-0">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleReset}
            disabled={isLoading || isSaving}
          >
            <RotateCcw className="w-4 h-4 mr-2" />
            {t('settings.resetToDefault')}
          </Button>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={isSaving}
            >
              {t('settings.cancel')}
            </Button>
            <Button
              size="sm"
              onClick={handleSave}
              disabled={isLoading || isSaving}
            >
              {isSaving ? t('settings.saving') : t('settings.save')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
