import { ArrowUpDown, ChevronRight, Compass, Sparkles, Swords, Target, Users } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Link } from 'react-router'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { Page, PageHeader } from '@/components/ui/Page'
import { t } from '@/i18n/ru'

function IconTile({ icon: Icon }: { icon: LucideIcon }) {
  return (
    <span className="flex size-12 shrink-0 items-center justify-center rounded-[14px] bg-surface-2 text-accent-2">
      <Icon size={24} aria-hidden />
    </span>
  )
}

// Хаб мини‑игр (SPEC 5.6): игры и квизы.
export default function PlayPage() {
  const play = t.play
  return (
    <Page title={play.title}>
      <PageHeader
        title={play.title}
        subtitle={play.subtitle}
        icon={<Target size={30} className="text-accent-2" aria-hidden />}
      />

      <div className="space-y-3">
        <Card padding="none" className="p-5">
          <div className="flex items-center gap-4">
            <IconTile icon={Swords} />
            <h2 className="text-card-title flex-1">{play.draftBattle.title}</h2>
            <Chip tone="outline" size="sm">
              {play.draftBattle.badge}
            </Chip>
          </div>
          <p className="mt-3 text-[15px] text-muted">{play.draftBattle.text}</p>
          <Button variant="accent2" fullWidth className="mt-4" disabled>
            {play.draftBattle.soon}
          </Button>
        </Card>

        <Card padding="none" className="p-5">
          <div className="flex items-center gap-4">
            <IconTile icon={ArrowUpDown} />
            <h2 className="text-card-title flex-1">{play.higherLower.title}</h2>
          </div>
          <p className="mt-3 text-[15px] text-muted">{play.higherLower.text}</p>
          <ButtonLink to="/play/higher-lower" variant="accent2" fullWidth className="mt-4">
            {play.higherLower.cta} →
          </ButtonLink>
        </Card>

        <Card padding="none" className="p-5">
          <div className="flex items-center gap-4">
            <IconTile icon={Users} />
            <h2 className="text-card-title flex-1">{play.party.title}</h2>
          </div>
          <p className="mt-3 text-[15px] text-muted">{play.party.text}</p>
          <ButtonLink to="/play/party" variant="accent2" fullWidth className="mt-4">
            {play.party.cta} →
          </ButtonLink>
        </Card>
      </div>

      <h2 className="caps mb-3 mt-8">{play.quizzes}</h2>
      <div className="space-y-3">
        <QuizRow
          to="/play/quiz/position"
          icon={Compass}
          title={play.quizPosition.title}
          subtitle={play.quizPosition.subtitle}
        />
        <QuizRow
          to="/play/quiz/hero"
          icon={Sparkles}
          title={play.quizHero.title}
          subtitle={play.quizHero.subtitle}
        />
      </div>
    </Page>
  )
}

function QuizRow({
  to,
  icon,
  title,
  subtitle,
}: {
  to: string
  icon: LucideIcon
  title: string
  subtitle: string
}) {
  return (
    <Link
      to={to}
      className="flex items-center gap-4 rounded-[16px] border border-line bg-surface p-4 transition-colors hover:bg-surface-2"
    >
      <IconTile icon={icon} />
      <div className="min-w-0 flex-1">
        <div className="text-card-title">{title}</div>
        <div className="text-[14px] text-muted">{subtitle}</div>
      </div>
      <ChevronRight size={20} className="shrink-0 text-muted" aria-hidden />
    </Link>
  )
}
