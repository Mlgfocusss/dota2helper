import type { ComponentType } from 'react'
import { createBrowserRouter, type RouteObject } from 'react-router'
import { AppLayout } from './layout/AppLayout'
import { AppSplash, RouteError } from './layout/RouteError'

// Каждая страница — отдельный чанк; роутер ждёт загрузки кода до смены экрана,
// поэтому восстановление скролла при «назад» работает и для ленивых страниц.
const page = (load: () => Promise<{ default: ComponentType }>) => async () => ({
  Component: (await load()).default,
})

const pages: RouteObject[] = [
  { index: true, lazy: page(() => import('@/pages/HomePage')) },
  { path: 'news', lazy: page(() => import('@/pages/NewsListPage')) },
  { path: 'news/:gid', lazy: page(() => import('@/pages/NewsPage')) },
  { path: 'feedback', lazy: page(() => import('@/pages/FeedbackPage')) },

  { path: 'play', lazy: page(() => import('@/pages/play/PlayPage')) },
  { path: 'play/higher-lower', lazy: page(() => import('@/pages/play/HigherLowerPage')) },
  { path: 'play/quiz/position', lazy: page(() => import('@/pages/play/QuizPositionPage')) },
  { path: 'play/quiz/hero', lazy: page(() => import('@/pages/play/QuizHeroPage')) },
  { path: 'play/party', lazy: page(() => import('@/pages/play/PartySearchPage')) },

  { path: 'coach', lazy: page(() => import('@/pages/coach/CoachPage')) },
  { path: 'coach/connect', lazy: page(() => import('@/pages/coach/CoachConnectPage')) },
  { path: 'coach/summary', lazy: page(() => import('@/pages/coach/CoachSummaryPage')) },
  { path: 'coach/matches/:matchId', lazy: page(() => import('@/pages/coach/MatchPage')) },
  { path: 'coach/deep', lazy: page(() => import('@/pages/coach/DeepAnalysisPage')) },
  { path: 'coach/analysis/:id', lazy: page(() => import('@/pages/coach/AnalysisPage')) },

  { path: 'reviews', lazy: page(() => import('@/pages/reviews/ReviewsPage')) },

  { path: 'tools', lazy: page(() => import('@/pages/tools/ToolsPage')) },
  { path: 'tools/drafter', lazy: page(() => import('@/pages/tools/DrafterPage')) },
  { path: 'tools/drafter/history', lazy: page(() => import('@/pages/tools/DrafterHistoryPage')) },
  { path: 'tools/drafter/top', lazy: page(() => import('@/pages/tools/DrafterTopPage')) },
  { path: 'tools/heroes', lazy: page(() => import('@/pages/tools/HeroesPage')) },
  { path: 'tools/heroes/:heroId', lazy: page(() => import('@/pages/tools/HeroPage')) },

  { path: 'profile', lazy: page(() => import('@/pages/profile/ProfilePage')) },
  { path: 'profile/party-card', lazy: page(() => import('@/pages/profile/PartyCardPage')) },

  { path: 'dev/ui', lazy: page(() => import('@/pages/dev/UiKitPage')) },

  { path: '*', lazy: page(() => import('@/pages/NotFoundPage')) },
]

export const router = createBrowserRouter([
  {
    path: '/',
    Component: AppLayout,
    ErrorBoundary: RouteError,
    HydrateFallback: AppSplash,
    // ошибка внутри страницы показывается в layout — шапка и навигация остаются
    children: [{ ErrorBoundary: RouteError, children: pages }],
  },
])
