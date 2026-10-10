type IconProps = { size?: number; className?: string }

/** Упрощённый знак Steam для кнопки входа. */
export function SteamIcon({ size = 20, className }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.9}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className={className}
    >
      <circle cx="12" cy="12" r="9.5" />
      <circle cx="15.2" cy="9.3" r="2.6" />
      <circle cx="8.7" cy="15.4" r="1.9" />
      <path d="M2.9 13.4 7 15M10.2 14.3l3.1-2.6" />
    </svg>
  )
}
