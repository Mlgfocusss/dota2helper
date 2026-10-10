// Ключи TanStack Query — только здесь, чтобы инвалидация не расползалась по коду.

export const keys = {
  me: ['me'] as const,
  config: ['config'] as const,
  heroes: ['heroes'] as const,
  patch: ['patch'] as const,
  metaPositions: (limit: number) => ['meta', 'positions', limit] as const,
  news: (limit: number) => ['news', 'list', limit] as const,
  newsItem: (gid: string) => ['news', 'item', gid] as const,
  newsComments: (gid: string) => ['news', 'comments', gid] as const,
  patchNotes: (version: string) => ['patch-notes', version] as const,
  coach: (accountId: number) => ['coach', accountId] as const,
  coachSummary: (accountId: number) => ['coach', accountId, 'summary'] as const,
  coachMatches: (accountId: number, filter: string) =>
    ['coach', accountId, 'matches', filter] as const,
  match: (matchId: number, accountId: number | null) => ['match', matchId, accountId] as const,
  heroPage: (heroId: number) => ['hero', heroId] as const,
  higherLower: (mode: string) => ['higher-lower', mode] as const,
  gameRecords: ['games', 'records'] as const,
  lastPosition: ['quiz', 'last-position'] as const,
  quiz: (quizId: string) => ['quiz', quizId] as const,
  drafterMatrix: ['drafter', 'matrix'] as const,
  drafterHistory: ['drafter', 'history'] as const,
  drafterLeaderboard: (month: string) => ['drafter', 'leaderboard', month] as const,
  partyMe: ['party', 'me'] as const,
  partySearchAll: ['party', 'search'] as const,
  partySearch: (filters: string) => ['party', 'search', filters] as const,
  profileStats: ['profile', 'stats'] as const,
  analyses: ['analyses'] as const,
  analysisList: ['analyses', 'list'] as const,
  analysisQuota: ['analyses', 'quota'] as const,
  analysis: (id: string) => ['analyses', id] as const,
}

/** staleTime по типам данных (SPEC 11). */
export const STALE = {
  constants: Infinity,
  meta: 30 * 60_000,
  summary: 2 * 60_000,
  me: 60_000,
} as const
