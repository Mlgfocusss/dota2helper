import {
  forwardRef,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react'
import { cn } from '@/lib/cn'

type FieldProps = {
  label: string
  htmlFor: string
  /** Подсказка под полем; заменяется ошибкой, если она есть */
  hint?: ReactNode
  error?: string
  /** Счётчик символов справа от подписи */
  counter?: { value: number; max: number }
  /** Подпись только для скринридера — когда заголовок уже есть рядом */
  hideLabel?: boolean
  className?: string
  children: ReactNode
}

/** Подпись, поле, подсказка или ошибка и счётчик символов. */
export function Field({
  label,
  htmlFor,
  hint,
  error,
  counter,
  hideLabel,
  className,
  children,
}: FieldProps) {
  return (
    <div className={className}>
      <div className="mb-2 flex items-baseline gap-3">
        <label htmlFor={htmlFor} className={cn('text-[14px] font-medium', hideLabel && 'sr-only')}>
          {label}
        </label>
        {counter && (
          <span
            className={cn(
              'ml-auto font-mono text-[12px] tabular-nums',
              counter.value > counter.max ? 'text-loss' : 'text-dim',
            )}
          >
            {counter.value} / {counter.max}
          </span>
        )}
      </div>
      {children}
      {error ? (
        <p role="alert" className="mt-1.5 text-[13px] text-loss">
          {error}
        </p>
      ) : (
        hint && <p className="mt-1.5 text-[13px] text-muted">{hint}</p>
      )}
    </div>
  )
}

const control =
  'w-full rounded-[12px] border border-line bg-bg px-4 text-[15px] text-fg placeholder:text-dim ' +
  'transition-[border-color,box-shadow] outline-none hover:border-muted/50 ' +
  'focus:border-accent-2 focus:ring-3 focus:ring-accent-2/20 ' +
  'aria-invalid:border-loss disabled:opacity-50'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...rest }, ref) {
    return <input ref={ref} className={cn(control, 'h-12', className)} {...rest} />
  },
)

/** Нативный выпадающий список в стиле полей (на мобиле открывает системный выбор). */
export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  function Select({ className, children, ...rest }, ref) {
    return (
      <select ref={ref} className={cn(control, 'h-12 cursor-pointer pr-3', className)} {...rest}>
        {children}
      </select>
    )
  },
)

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className, ...rest }, ref) {
  return (
    <textarea
      ref={ref}
      className={cn(control, 'min-h-[168px] resize-y py-3 leading-relaxed', className)}
      {...rest}
    />
  )
})
