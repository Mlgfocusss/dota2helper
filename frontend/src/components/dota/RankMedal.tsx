import { useId } from 'react'
import { parseRankTier } from '@/lib/ranks'

// Цвета медалей: калибровка, Рекрут … Титан. Своя отрисовка вместо ассетов Valve (SPEC 3.4).
const COLORS = [
  '#6B6C75',
  '#9C8A78',
  '#7DA08C',
  '#B98B58',
  '#6C9BD4',
  '#D2AC52',
  '#C8683D',
  '#B7A4F0',
  '#E8E3C8',
]

type RankMedalProps = {
  rankTier: number | null | undefined
  leaderboardRank?: number | null
  size?: number
  /** Звёзды под медалью; при маленьком размере скрываем */
  showStars?: boolean
  className?: string
}

/** Бейдж ранга: медаль с номером и звёзды слоями. */
export function RankMedal({
  rankTier,
  leaderboardRank,
  size = 48,
  showStars = true,
  className,
}: RankMedalProps) {
  const rank = parseRankTier(rankTier, leaderboardRank)
  const color = COLORS[rank.medal] ?? COLORS[0]
  const gradientId = useId()
  const stars = showStars && rank.medal > 0 && rank.medal < 8 ? rank.stars : 0

  return (
    <svg
      viewBox="0 0 48 52"
      width={size}
      height={size * (52 / 48)}
      role="img"
      aria-label={rank.label}
      className={className}
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={color} stopOpacity="0.95" />
          <stop offset="1" stopColor={color} stopOpacity="0.45" />
        </linearGradient>
      </defs>
      <path
        d="M24 2 43 12v17L24 42 5 29V12z"
        fill={`url(#${gradientId})`}
        stroke={color}
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path
        d="M24 8 37.5 15v12L24 36 10.5 27V15z"
        fill="none"
        stroke="#0A0A0C"
        strokeOpacity="0.35"
        strokeWidth="1.2"
      />
      <text
        x="24"
        y="27"
        textAnchor="middle"
        fontSize="16"
        fontWeight="700"
        fill="#0A0A0C"
        fillOpacity="0.8"
        fontFamily="Inter, sans-serif"
      >
        {rank.medal === 0 ? '?' : rank.medal}
      </text>
      {Array.from({ length: stars }, (_, i) => (
        <circle key={i} cx={24 + (i - (stars - 1) / 2) * 6} cy="47.5" r="2.2" fill={color} />
      ))}
    </svg>
  )
}
