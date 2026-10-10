import { LoaderCircle } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useLocation } from 'react-router'
import { steamLoginUrl } from '@/api/endpoints/system'
import { SteamIcon } from '@/components/icons/SteamIcon'
import { ButtonAnchor } from '@/components/ui/Button'
import type { ButtonSize } from '@/components/ui/button-styles'
import { t } from '@/i18n/ru'

type SteamLoginButtonProps = {
  /** Куда вернуться после входа; по умолчанию — текущая страница */
  returnTo?: string
  size?: ButtonSize
  fullWidth?: boolean
  className?: string
  children?: ReactNode
  /** Вызывается при клике, перед переходом в Steam */
  onStart?: () => void
}

/** Текущий адрес без ?login=…, чтобы после входа не всплыло старое уведомление. */
function useCurrentPath(): string {
  const { pathname, search } = useLocation()
  const params = new URLSearchParams(search)
  params.delete('login')
  const qs = params.toString()
  return qs ? `${pathname}?${qs}` : pathname
}

/** Кнопка входа через Steam: полный переход в той же вкладке, после клика — спиннер. */
export function SteamLoginButton({
  returnTo,
  size = 'md',
  fullWidth,
  className,
  children = t.header.login,
  onStart,
}: SteamLoginButtonProps) {
  const currentPath = useCurrentPath()
  const [pending, setPending] = useState(false)

  // вернулись кнопкой «назад» со страницы Steam — страница из bfcache, снимаем спиннер
  useEffect(() => {
    const onPageShow = (e: PageTransitionEvent) => {
      if (e.persisted) setPending(false)
    }
    window.addEventListener('pageshow', onPageShow)
    return () => window.removeEventListener('pageshow', onPageShow)
  }, [])

  const iconSize = size === 'xl' || size === 'lg' ? 22 : 18
  return (
    <ButtonAnchor
      href={steamLoginUrl(returnTo ?? currentPath)}
      variant="accent"
      size={size}
      fullWidth={fullWidth}
      className={className}
      aria-busy={pending || undefined}
      aria-disabled={pending || undefined}
      onClick={(e) => {
        if (pending) e.preventDefault()
        else {
          setPending(true)
          onStart?.()
        }
      }}
    >
      {pending ? (
        <LoaderCircle size={iconSize} className="animate-spin" aria-hidden />
      ) : (
        <SteamIcon size={iconSize} />
      )}
      {children}
    </ButtonAnchor>
  )
}
