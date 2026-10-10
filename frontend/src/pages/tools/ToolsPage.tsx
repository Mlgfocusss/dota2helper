import { ChevronRight, Crosshair, Shield, Wrench } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Link } from 'react-router'
import { Page, PageHeader } from '@/components/ui/Page'
import { t } from '@/i18n/ru'

function ToolCard({
  to,
  icon: Icon,
  title,
  text,
}: {
  to: string
  icon: LucideIcon
  title: string
  text: string
}) {
  return (
    <Link
      to={to}
      className="group flex flex-col rounded-[16px] border border-line bg-surface p-6 transition-colors hover:bg-surface-2"
    >
      <span className="flex size-14 items-center justify-center rounded-[16px] bg-surface-2 text-accent-2 transition-colors group-hover:bg-bg">
        <Icon size={28} aria-hidden />
      </span>
      <h2 className="text-section-title mt-5">{title}</h2>
      <p className="mt-2 flex-1 text-[15px] text-muted">{text}</p>
      <span className="mt-5 flex items-center gap-1 text-[15px] font-semibold text-accent-2">
        {t.tools.open}
        <ChevronRight
          size={18}
          aria-hidden
          className="transition-transform group-hover:translate-x-0.5"
        />
      </span>
    </Link>
  )
}

// Хаб инструментов (SPEC 4.1): две большие карточки рядом.
export default function ToolsPage() {
  return (
    <Page title={t.tools.title}>
      <PageHeader
        title={t.tools.title}
        subtitle={t.tools.subtitle}
        icon={<Wrench size={30} className="text-accent-2" aria-hidden />}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <ToolCard
          to="/tools/drafter"
          icon={Crosshair}
          title={t.tools.drafter.title}
          text={t.tools.drafter.text}
        />
        <ToolCard
          to="/tools/heroes"
          icon={Shield}
          title={t.tools.heroes.title}
          text={t.tools.heroes.text}
        />
      </div>
    </Page>
  )
}
