import { cn } from '@/lib/cn'

export type ButtonVariant = 'accent' | 'accent2' | 'surface' | 'outline' | 'ghost' | 'danger'
export type ButtonSize = 'sm' | 'md' | 'lg' | 'xl' | 'icon' | 'icon-sm'

export type ButtonStyleProps = {
  variant?: ButtonVariant
  size?: ButtonSize
  fullWidth?: boolean
}

// Hover — белая подложка 6% под содержимым (осветление на 6%, SPEC 2.5)
const base =
  'relative isolate inline-flex shrink-0 select-none items-center justify-center gap-2 overflow-hidden whitespace-nowrap font-semibold ' +
  'transition-[color,background-color,border-color,opacity,transform] duration-150 active:scale-[0.98] ' +
  'disabled:pointer-events-none disabled:opacity-45 aria-disabled:pointer-events-none aria-disabled:opacity-45 ' +
  'after:pointer-events-none after:absolute after:inset-0 after:-z-10 after:bg-white after:opacity-0 after:transition-opacity hover:after:opacity-[0.06] ' +
  '[&_svg]:shrink-0'

const variants: Record<ButtonVariant, string> = {
  accent: 'bg-accent text-on-accent',
  accent2: 'bg-accent-2 text-on-accent',
  surface: 'bg-surface-2 text-fg',
  outline: 'border border-line bg-transparent text-fg',
  ghost: 'bg-transparent text-muted hover:text-fg',
  danger: 'border border-loss/40 bg-transparent text-loss',
}

const sizes: Record<ButtonSize, string> = {
  sm: 'h-9 rounded-[12px] px-3.5 text-[14px]',
  md: 'h-12 rounded-[14px] px-5 text-[15px]',
  lg: 'h-14 rounded-[14px] px-6 text-[16px]',
  xl: 'h-16 rounded-[14px] px-6 text-[17px]',
  icon: 'size-11 rounded-[12px]',
  'icon-sm': 'size-9 rounded-[10px]',
}

export function buttonClass({ variant = 'accent', size = 'md', fullWidth }: ButtonStyleProps = {}) {
  return cn(base, variants[variant], sizes[size], fullWidth && 'w-full')
}
