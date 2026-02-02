import { Button } from '@/components/ui/button'
import { domain } from '@/../wailsjs/go/models'
import { useTranslation } from 'react-i18next'

interface TargetsPanelProps {
  targets: domain.TargetInfo[]
  attachedTargetId: string | null
  onToggle: (id: string) => void
  isConnected: boolean
}

export function TargetsPanel({ 
  targets, 
  attachedTargetId, 
  onToggle,
  isConnected 
}: TargetsPanelProps) {
  const { t } = useTranslation()

  if (!isConnected) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        {t('targets.connectFirst')}
      </div>
    )
  }

  if (targets.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        {t('targets.noTargets')}
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
            variant={attachedTargetId === target.id ? "default" : "outline"}
            size="sm"
            onClick={() => onToggle(target.id)}
          >
            {attachedTargetId === target.id ? t('targets.attached') : t('targets.attach')}
          </Button>
        </div>
      ))}
    </div>
  )
}
