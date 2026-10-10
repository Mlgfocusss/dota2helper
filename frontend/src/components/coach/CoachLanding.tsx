import { motion, useReducedMotion } from 'framer-motion'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { Page } from '@/components/ui/Page'
import { t } from '@/i18n/ru'

/** Экран гостя в «Тренере»: вход через Steam или поиск профиля по ссылке (SPEC 5.2). */
export function CoachLanding() {
  const [params] = useSearchParams()
  // ?login=cancelled чистится в useAuthEffects — запоминаем при первом рендере
  const [cancelled] = useState(() => params.get('login') === 'cancelled')
  const [redirecting, setRedirecting] = useState(false)
  const status = redirecting
    ? t.coach.landing.redirecting
    : cancelled
      ? t.coach.landing.cancelled
      : null

  return (
    <Page width={1200} title={t.pages.coach.title}>
      <div className="relative grid min-h-[calc(100dvh-64px-28px-108px)] items-center gap-10 lg:grid-cols-2">
        <div className="relative z-10 mx-auto w-full max-w-[520px]">
          <h1 className="text-[56px] font-bold leading-[1.05] tracking-[-0.02em] max-md:text-[40px]">
            {t.coach.landing.title}
          </h1>
          <p className="mt-4 text-[18px] text-muted">{t.coach.landing.subtitle}</p>

          <div className="mt-10">
            <SteamLoginButton
              size="xl"
              fullWidth
              returnTo="/coach"
              onStart={() => setRedirecting(true)}
            >
              {t.coach.landing.login}
            </SteamLoginButton>
            <div className="mt-5 text-center">
              <Link
                to="/coach/connect"
                className="text-[15px] font-medium text-muted underline-offset-4 transition-colors hover:text-fg hover:underline"
              >
                {t.coach.landing.connect}
              </Link>
            </div>
            {status && (
              <p
                role="status"
                className="mt-6 rounded-[14px] border border-line bg-surface px-4 py-3 text-center text-[15px] text-muted"
              >
                {status}
              </p>
            )}
          </div>
        </div>

        <CoachArt />
      </div>
    </Page>
  )
}

/** Своя иллюстрация: тёмный градиент с красными и синими «частицами»; дышит, если не отключены анимации. */
function CoachArt() {
  const reduce = useReducedMotion()
  const breathe = (delay: number) =>
    reduce
      ? {}
      : {
          animate: { scale: [1, 1.12, 1], opacity: [0.55, 0.9, 0.55] },
          transition: { duration: 6, repeat: Infinity, ease: 'easeInOut' as const, delay },
        }

  return (
    <div
      aria-hidden
      className="pointer-events-none relative overflow-hidden rounded-[28px] border border-line bg-[radial-gradient(ellipse_at_50%_40%,#17181f_0%,#0c0c10_70%)] max-lg:absolute max-lg:inset-0 max-lg:z-0 max-lg:rounded-none max-lg:border-0 max-lg:opacity-35 lg:h-full lg:min-h-[480px]"
    >
      <motion.div
        className="absolute -left-10 top-[12%] size-[320px] rounded-full bg-[#e5484d] opacity-55 blur-[90px]"
        {...breathe(0)}
      />
      <motion.div
        className="absolute -right-8 bottom-[8%] size-[360px] rounded-full bg-[#3b6cf0] opacity-55 blur-[100px]"
        {...breathe(2)}
      />
      <svg
        viewBox="0 0 400 520"
        className="absolute inset-0 size-full"
        preserveAspectRatio="xMidYMid slice"
      >
        <g fill="none" strokeWidth="1.2" strokeLinecap="round">
          <path
            d="M-20 420 C 90 360 130 300 210 270 S 330 150 430 90"
            stroke="#e5484d"
            strokeOpacity="0.5"
          />
          <path
            d="M-20 470 C 110 420 170 350 240 320 S 340 220 430 170"
            stroke="#e5484d"
            strokeOpacity="0.25"
          />
          <path
            d="M-20 120 C 80 160 150 210 210 250 S 330 380 430 430"
            stroke="#5b8cff"
            strokeOpacity="0.5"
          />
          <path
            d="M-20 60 C 100 100 160 170 230 215 S 340 330 430 370"
            stroke="#5b8cff"
            strokeOpacity="0.25"
          />
        </g>
        <g>
          {PARTICLES.map(([x, y, r, red], i) => (
            <circle
              key={i}
              cx={x}
              cy={y}
              r={r}
              fill={red ? '#ff6b70' : '#7aa2ff'}
              fillOpacity={0.35 + (i % 4) * 0.12}
            />
          ))}
        </g>
      </svg>
    </div>
  )
}

// x, y, радиус, красная ли
const PARTICLES: [number, number, number, boolean][] = [
  [60, 90, 2.5, false],
  [120, 150, 1.6, true],
  [210, 70, 2, false],
  [300, 120, 3, true],
  [350, 210, 1.8, false],
  [90, 260, 2.2, true],
  [180, 230, 1.5, false],
  [260, 300, 2.8, true],
  [330, 340, 1.7, false],
  [70, 380, 2.4, false],
  [150, 430, 1.8, true],
  [240, 410, 2.6, false],
  [310, 470, 2, true],
  [40, 200, 1.4, true],
  [200, 340, 1.4, false],
  [370, 60, 2.2, true],
]
