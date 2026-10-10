/** Знак приложения: те же формы, что в favicon. */
export function LogoMark({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden>
      <rect width="32" height="32" rx="9" fill="var(--surface-2)" />
      <path d="M9 22.5 22.5 9" stroke="var(--accent-2)" strokeWidth="3" strokeLinecap="round" />
      <circle cx="11" cy="11" r="3" fill="var(--accent)" />
      <circle cx="21" cy="21" r="3" fill="var(--accent)" />
    </svg>
  )
}
