import { LoaderCircle } from 'lucide-react'
import { forwardRef, type AnchorHTMLAttributes, type ButtonHTMLAttributes } from 'react'
import { Link, type LinkProps } from 'react-router'
import { cn } from '@/lib/cn'
import { buttonClass, type ButtonStyleProps } from './button-styles'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  ButtonStyleProps & {
    /** Спиннер вместо иконки и блокировка повторного нажатия */
    loading?: boolean
  }

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant, size, fullWidth, loading, disabled, className, children, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(buttonClass({ variant, size, fullWidth }), className)}
      {...rest}
    >
      {loading && <LoaderCircle size={18} className="animate-spin" aria-hidden />}
      {children}
    </button>
  )
})

type IconButtonProps = Omit<ButtonProps, 'size' | 'aria-label'> & {
  'aria-label': string
  size?: 'icon' | 'icon-sm'
}

/** Квадратная кнопка с иконкой; подпись для скринридера обязательна. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { variant = 'ghost', size = 'icon', ...rest },
  ref,
) {
  return <Button ref={ref} variant={variant} size={size} {...rest} />
})

type ButtonLinkProps = LinkProps & ButtonStyleProps

/** Внутренняя ссылка, оформленная как кнопка. */
export function ButtonLink({ variant, size, fullWidth, className, ...rest }: ButtonLinkProps) {
  return <Link className={cn(buttonClass({ variant, size, fullWidth }), className)} {...rest} />
}

type ButtonAnchorProps = AnchorHTMLAttributes<HTMLAnchorElement> & ButtonStyleProps

/** Внешняя ссылка или полный переход (например, вход через Steam). */
export function ButtonAnchor({ variant, size, fullWidth, className, ...rest }: ButtonAnchorProps) {
  return <a className={cn(buttonClass({ variant, size, fullWidth }), className)} {...rest} />
}
