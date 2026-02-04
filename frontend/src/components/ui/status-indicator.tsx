import { cn } from '@/lib/utils'
import { ChevronRight } from 'lucide-react'

type StepStatus = 'completed' | 'current' | 'pending'

interface StatusStep {
  key: string
  label: string
  status: StepStatus
}

interface StatusIndicatorProps {
  steps: StatusStep[]
}

export function StatusIndicator({ steps }: StatusIndicatorProps) {
  return (
    <div className="flex items-center gap-1">
      {steps.map((step, index) => {
        const isCompleted = step.status === 'completed'
        const isCurrent = step.status === 'current'
        const isPending = step.status === 'pending'
        
        return (
          <div key={step.key} className="flex items-center gap-1">
            <div
              className={cn(
                'px-2 py-1 text-xs font-medium transition-all',
                'flex items-center justify-center gap-1.5',
                'bg-muted text-foreground rounded'
              )}
            >
              <span
                className={cn(
                  'w-1.5 h-1.5 rounded-full flex-shrink-0 transition-all',
                  isCompleted && 'bg-green-400 shadow-[0_0_6px_rgba(74,222,128,0.8)]',
                  isCurrent && 'bg-orange-400 shadow-[0_0_6px_rgba(251,146,60,0.8)]',
                  isPending && 'bg-gray-400'
                )}
              />
              <span className="whitespace-nowrap">{step.label}</span>
            </div>
            {index < steps.length - 1 && (
              <ChevronRight className="w-3 h-3 text-muted-foreground flex-shrink-0" />
            )}
          </div>
        )
      })}
    </div>
  )
}
