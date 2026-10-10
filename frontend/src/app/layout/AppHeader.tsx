import { ArrowLeft } from 'lucide-react'
import { Link, useLocation, useNavigate } from 'react-router'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { LogoMark } from '@/components/icons/LogoMark'
import { Avatar } from '@/components/ui/Avatar'
import { IconButton } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { APP_NAME } from '@/lib/config'
import { useScrolled } from '@/lib/hooks'
import { isTabRoot, parentPath } from '@/lib/routes'

/** Глобальная шапка: «назад» на вложенных страницах, логотип, аватар или вход. */
export function AppHeader() {
  const location = useLocation()
  const navigate = useNavigate()
  const scrolled = useScrolled(0)
  const showBack = !isTabRoot(location.pathname)

  const goBack = () => {
    // key 'default' — это первая страница сессии: истории внутри сайта нет
    if (location.key !== 'default') navigate(-1)
    else navigate(parentPath(location.pathname))
  }

  return (
    <header
      className={cn(
        'sticky top-0 z-40 h-16 border-b bg-bg/80 backdrop-blur-md transition-colors duration-200',
        scrolled ? 'border-line' : 'border-transparent',
      )}
    >
      <div className="mx-auto flex h-full w-[min(1200px,calc(100%-48px))] items-center gap-2 max-md:w-[calc(100%-32px)]">
        {showBack && (
          <IconButton aria-label={t.header.back} onClick={goBack} className="-ml-2.5 text-fg">
            <ArrowLeft size={22} />
          </IconButton>
        )}
        <Link
          to="/"
          aria-label={`${APP_NAME} — ${t.header.home}`}
          className="flex items-center gap-2.5 rounded-[10px] py-1 pr-1 transition-opacity hover:opacity-85"
        >
          <LogoMark size={28} />
          <span className="text-[18px] font-semibold tracking-[-0.01em]">{APP_NAME}</span>
        </Link>
        <div className="ml-auto flex items-center">
          <HeaderAccount />
        </div>
      </div>
    </header>
  )
}

function HeaderAccount() {
  const { data, isPending } = useMe()

  if (isPending) return <Skeleton radius="full" className="size-9" />

  const user = data?.user
  if (user) {
    return (
      <Link
        to="/profile"
        aria-label={t.header.profile}
        className="rounded-full transition-opacity hover:opacity-85"
      >
        <Avatar src={user.avatar} name={user.persona_name} size={36} />
      </Link>
    )
  }

  return (
    <SteamLoginButton size="sm">
      <span className="max-sm:hidden">{t.header.login}</span>
      <span className="sm:hidden">{t.header.loginShort}</span>
    </SteamLoginButton>
  )
}
