import { useState, useEffect } from 'react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ChevronDown, Filter } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface ActionButton {
  label: string
  onClick: () => void
  disabled?: boolean
  variant?: 'default' | 'destructive' | 'outline' | 'ghost'
  icon?: ReactNode
  iconOnly?: boolean
}

interface PageHeaderProps {
  title: string
  filters?: ReactNode
  actions?: ActionButton[]
  activeFiltersCount?: number
  hasActiveFilters?: boolean
}

export function PageHeader({
  title,
  filters,
  actions,
  activeFiltersCount = 0,
  hasActiveFilters,
}: PageHeaderProps) {
  const [filtersOpen, setFiltersOpen] = useState(false)

  const shouldAutoOpen = hasActiveFilters ?? activeFiltersCount > 0

  useEffect(() => {
    if (shouldAutoOpen) {
      setFiltersOpen(true)
    }
  }, [])

  const hasFilters = !!filters

  return (
    <Collapsible open={filtersOpen} onOpenChange={setFiltersOpen}>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">{title}</h1>

        <div className="flex items-center gap-2">
          {hasFilters && (
            <CollapsibleTrigger asChild>
              <Button variant="outline" size="sm">
                <Filter className="h-4 w-4 mr-1" />
                Фильтры
                {activeFiltersCount > 0 && (
                  <Badge variant="secondary" className="ml-1">{activeFiltersCount}</Badge>
                )}
                <ChevronDown className={cn("h-4 w-4 ml-1 transition-transform", filtersOpen && "rotate-180")} />
              </Button>
            </CollapsibleTrigger>
          )}

          {actions && actions.map((action, idx) => (
            action.iconOnly && action.icon ? (
              <Tooltip key={idx}>
                <TooltipTrigger asChild>
                  <Button
                    variant={action.variant ?? 'outline'}
                    size="icon"
                    onClick={action.onClick}
                    disabled={action.disabled}
                  >
                    {action.icon}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{action.label}</p>
                </TooltipContent>
              </Tooltip>
            ) : (
              <Button
                key={idx}
                variant={action.variant ?? 'outline'}
                size="sm"
                onClick={action.onClick}
                disabled={action.disabled}
              >
                {action.icon && <span className="mr-1">{action.icon}</span>}
                {action.label}
              </Button>
            )
          ))}
        </div>
      </div>

      {hasFilters && (
        <CollapsibleContent>
          <div className="mt-4 p-4 border rounded-lg bg-muted/30 space-y-3">
            {filters}
          </div>
        </CollapsibleContent>
      )}
    </Collapsible>
  )
}
