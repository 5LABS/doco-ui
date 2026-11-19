import type { ComponentProps } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badge = cva('inline-flex items-center rounded-md border px-2 py-1 text-xs font-medium', {
  variants: {
    variant: {
      secondary: 'border-transparent bg-slate-100 text-slate-700',
      outline: 'border-slate-300 text-slate-600',
    },
  },
  defaultVariants: { variant: 'secondary' },
})

export function Badge({ className, variant, ...props }: ComponentProps<'span'> & VariantProps<typeof badge>) {
  return <span data-slot="badge" className={cn(badge({ variant }), className)} {...props} />
}
