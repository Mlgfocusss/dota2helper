import { useId } from 'react'
import type { AnalysisResult } from '@/api/types'
import { t } from '@/i18n/ru'

type MiniMapProps = {
  points: AnalysisResult['death_points']
  className?: string
}

/**
 * Схематичная мини-карта (своя отрисовка, не скриншот игры): база Света внизу слева,
 * Тьмы — вверху справа, три линии и река по диагонали. Точки — места смертей игрока в драках.
 */
export function MiniMap({ points, className }: MiniMapProps) {
  const id = useId()
  const max = Math.max(1, ...points.map((p) => p.n))
  return (
    <figure className={className}>
      <svg
        viewBox="0 0 100 100"
        role="img"
        aria-label={t.analysis.result.map}
        className="aspect-square w-full rounded-[14px] border border-line bg-bg"
      >
        <defs>
          <radialGradient id={`${id}-glow`}>
            <stop offset="0" stopColor="var(--loss-strong)" stopOpacity="0.85" />
            <stop offset="1" stopColor="var(--loss-strong)" stopOpacity="0" />
          </radialGradient>
        </defs>

        {/* лес и стороны карты */}
        <polygon points="0,100 0,0 100,100" fill="var(--radiant)" opacity="0.05" />
        <polygon points="100,0 0,0 100,100" fill="var(--dire)" opacity="0.05" />

        {/* река: поперёк оси баз, из левого верхнего угла в правый нижний */}
        <path d="M-5 8 L8 -5 L105 92 L92 105 Z" fill="#3a5a8a" opacity="0.28" />

        {/* линии: верхняя, средняя, нижняя */}
        <g
          fill="none"
          stroke="var(--text-dim)"
          strokeOpacity="0.5"
          strokeWidth="1.4"
          strokeLinecap="round"
        >
          <path d="M10 88 L10 12 L88 12" />
          <path d="M14 86 L86 14" />
          <path d="M12 90 L88 90 L88 14" />
        </g>

        {/* базы */}
        <circle cx="12" cy="88" r="7" fill="var(--radiant)" opacity="0.35" />
        <circle cx="88" cy="12" r="7" fill="var(--dire)" opacity="0.35" />

        {/* смерти: чем чаще, тем крупнее и ярче */}
        {points.map((p, i) => {
          const weight = p.n / max
          return (
            <g key={i} transform={`translate(${p.x * 100} ${p.y * 100})`}>
              <circle r={3 + weight * 5} fill={`url(#${id}-glow)`} opacity={0.5 + weight * 0.5} />
              <circle
                r={1.1 + weight * 0.9}
                fill="var(--loss)"
                stroke="var(--bg)"
                strokeWidth="0.4"
              />
            </g>
          )
        })}
      </svg>
      <figcaption className="mt-2 text-[12px] text-dim">
        {points.length > 0 ? t.analysis.result.mapLegend : t.analysis.result.mapEmpty}
      </figcaption>
    </figure>
  )
}
