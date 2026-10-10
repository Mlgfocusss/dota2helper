import { cn } from '@/lib/cn'

type ToggleProps = {
  checked: boolean
  onChange: (checked: boolean) => void
  /** Подпись для скринридера, если рядом нет видимого label */
  label?: string
  id?: string
  disabled?: boolean
  className?: string
}

/** Переключатель‑свитч. */
export function Toggle({ checked, onChange, label, id, disabled, className }: ToggleProps) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        'relative inline-flex h-7 w-12 shrink-0 items-center rounded-full border transition-colors disabled:opacity-45',
        checked ? 'border-accent-2 bg-accent-2' : 'border-line bg-surface-2',
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'size-5 rounded-full shadow-sm transition-[translate,background-color] duration-200',
          checked ? 'translate-x-[22px] bg-white' : 'translate-x-[3px] bg-muted',
        )}
      />
    </button>
  )
}
