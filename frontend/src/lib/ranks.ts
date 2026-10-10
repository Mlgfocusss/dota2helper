import { t } from '@/i18n/ru'

export type RankInfo = {
  /** 0 — калибровка, 1–8 — Рекрут…Титан */
  medal: number
  /** 0–5 */
  stars: number
  /** «Легенда» */
  name: string
  /** «Легенда 1», «Титан #123», «Калибровка» */
  label: string
}

export const RANK_MEDALS = [0, 1, 2, 3, 4, 5, 6, 7, 8] as const

export function rankName(medal: number): string {
  return t.ranks[medal] ?? t.ranks[0]
}

/**
 * rank_tier из OpenDota: десятки — медаль (1–8), единицы — звёзды (1–5).
 * У Титана вместо звёзд может быть место в лидерборде.
 */
export function parseRankTier(
  rankTier: number | null | undefined,
  leaderboardRank?: number | null,
): RankInfo {
  if (!rankTier || rankTier < 10) {
    const name = rankName(0)
    return { medal: 0, stars: 0, name, label: name }
  }
  const medal = Math.min(8, Math.floor(rankTier / 10))
  const stars = Math.min(5, rankTier % 10)
  const name = rankName(medal)
  let label = stars > 0 && medal < 8 ? `${name} ${stars}` : name
  if (medal === 8 && leaderboardRank) label = `${name} #${leaderboardRank}`
  return { medal, stars, name, label }
}
