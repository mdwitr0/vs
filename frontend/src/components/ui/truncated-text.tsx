import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface TruncatedTextProps {
  text: string
  className?: string
  maxWidth?: string
  as?: 'span' | 'div' | 'a'
  href?: string
}

export function TruncatedText({
  text,
  className,
  maxWidth = 'max-w-[200px]',
  as: Component = 'span',
  href,
}: TruncatedTextProps) {
  const content = (
    <Component
      {...(href && { href, target: '_blank', rel: 'noopener noreferrer' })}
      className={cn('truncate block', maxWidth, className)}
    >
      {text}
    </Component>
  )

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {content}
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-md break-all">
        <p>{text}</p>
      </TooltipContent>
    </Tooltip>
  )
}
