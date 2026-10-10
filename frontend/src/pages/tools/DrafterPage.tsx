import { History, Trophy } from 'lucide-react'
import { useSearchParams } from 'react-router'
import { AnalysisTab } from '@/components/drafter/AnalysisTab'
import { TrainingTab } from '@/components/drafter/TrainingTab'
import { ButtonLink } from '@/components/ui/Button'
import { Page } from '@/components/ui/Page'
import { Segmented } from '@/components/ui/Segmented'
import { t } from '@/i18n/ru'

type Tab = 'analysis' | 'training'

const TAB_ITEMS = [
  { value: 'analysis' as const, label: t.drafter.tabs.analysis },
  { value: 'training' as const, label: t.drafter.tabs.training },
]

// Драфтер (SPEC 4.2): «Анализ» и «Тренировка» — основная колонка 760 и правая панель 376.
export default function DrafterPage() {
  const [params, setParams] = useSearchParams()
  const tab: Tab = params.get('tab') === 'training' ? 'training' : 'analysis'

  const setTab = (next: Tab) =>
    setParams(
      (prev) => {
        const out = new URLSearchParams(prev)
        out.set('tab', next)
        return out
      },
      { replace: true, preventScrollReset: true },
    )

  return (
    <Page width={1160} title={t.drafter.caption}>
      <div className="mb-4 flex items-center gap-3">
        <h1 className="caps">{t.drafter.caption}</h1>
        <div className="ml-auto flex gap-2">
          <ButtonLink
            to="/tools/drafter/history"
            variant="surface"
            size="icon"
            aria-label={t.drafter.historyLabel}
            title={t.drafter.historyLabel}
          >
            <History size={20} aria-hidden />
          </ButtonLink>
          <ButtonLink
            to="/tools/drafter/top"
            variant="surface"
            size="icon"
            aria-label={t.drafter.topLabel}
            title={t.drafter.topLabel}
          >
            <Trophy size={20} aria-hidden />
          </ButtonLink>
        </div>
      </div>

      <Segmented items={TAB_ITEMS} value={tab} onChange={setTab} aria-label={t.drafter.tabsLabel} />

      <div className="mt-6 grid gap-6 xl:grid-cols-[760px_376px] xl:justify-center">
        {tab === 'analysis' ? <AnalysisTab key="analysis" /> : <TrainingTab key="training" />}
      </div>
    </Page>
  )
}
