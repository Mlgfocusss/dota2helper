import {
  Bell,
  Crosshair,
  Heart,
  Layers,
  MessageCircle,
  RotateCw,
  Search,
  Swords,
  Users,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { HeroIcon, HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { SteamIcon } from '@/components/icons/SteamIcon'
import { Accordion } from '@/components/ui/Accordion'
import { Avatar } from '@/components/ui/Avatar'
import { Button, ButtonLink, IconButton } from '@/components/ui/Button'
import { Card, CardLink } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { ProgressBar } from '@/components/ui/ProgressBar'
import { SectionHeader } from '@/components/ui/SectionHeader'
import { Segmented } from '@/components/ui/Segmented'
import { Sheet } from '@/components/ui/Sheet'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tabs } from '@/components/ui/Tabs'
import { Toggle } from '@/components/ui/Toggle'
import { Tooltip } from '@/components/ui/Tooltip'
import { toast } from '@/features/toast/store'
import { useHeroes, usePatch } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import {
  MINUS,
  formatDuration,
  formatNumber,
  formatRelativeTime,
  formatSigned,
  pluralize,
} from '@/lib/format'
import { POSITIONS, positionFull, positionShort } from '@/lib/positions'
import { parseRankTier } from '@/lib/ranks'

const SWATCHES = [
  'bg',
  'surface',
  'surface-2',
  'border',
  'text',
  'text-muted',
  'text-dim',
  'accent',
  'accent-2',
  'win',
  'win-strong',
  'loss',
  'loss-strong',
  'radiant',
  'dire',
  'pos1',
  'pos2',
  'pos3',
  'pos4',
  'pos5',
]

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mb-10">
      <SectionHeader label={title} className="mb-3" />
      {children}
    </section>
  )
}

/** Витрина дизайн‑системы для проверки компонентов. */
export default function UiKitPage() {
  const [tab, setTab] = useState<'all' | 'win' | 'loss'>('all')
  const [mode, setMode] = useState<'analysis' | 'training'>('analysis')
  const [period, setPeriod] = useState<'all' | 'patch'>('all')
  const [mic, setMic] = useState(true)
  const [discord, setDiscord] = useState(false)
  const [progress, setProgress] = useState(0.4)
  const [chips, setChips] = useState<string[]>(['ranked'])
  const [sheetOpen, setSheetOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [fourDaysAgo] = useState(() => Date.now() - 4 * 86_400_000)

  const toggleChip = (id: string) =>
    setChips((list) => (list.includes(id) ? list.filter((c) => c !== id) : [...list, id]))

  const fakeLoad = () => {
    setLoading(true)
    window.setTimeout(() => setLoading(false), 1500)
  }

  return (
    <Page width={1000} title={t.pages.uiKit.title}>
      <PageHeader
        title={t.pages.uiKit.title}
        subtitle="Токены, типографика и компоненты дизайн‑системы"
      />

      <Section title="Цвета">
        <div className="grid grid-cols-4 gap-3 sm:grid-cols-5 md:grid-cols-10">
          {SWATCHES.map((name) => (
            <div key={name} className="text-[12px] text-muted">
              <div
                className="mb-1.5 h-12 rounded-[10px] border border-line"
                style={{ background: `var(--${name})` }}
              />
              {name}
            </div>
          ))}
        </div>
      </Section>

      <Section title="Типографика">
        <Card className="space-y-3">
          <p className="text-page-title">Заголовок страницы 32</p>
          <p className="text-section-title">Заголовок секции 22</p>
          <p className="text-card-title">Заголовок карточки 18</p>
          <p>Базовый текст 16/1.5 — разбор матчей и понятный план роста.</p>
          <p className="text-[14px] text-muted">Вторичный текст 14px</p>
          <p className="caps">Подпись капсом</p>
          <p className="text-[52px] font-bold leading-none tabular-nums">70%</p>
          <p className="font-mono text-[20px] font-semibold">
            <span className="text-win-strong">{formatSigned(20)}</span>{' '}
            <span className="text-loss-strong">{formatSigned(-11.84)}</span>{' '}
            <span className="text-muted">{formatSigned(2.66)}</span>
          </p>
          <p className="text-[14px] text-muted">
            {formatNumber(1234567)} · {formatDuration(2380)} · {formatDuration(3735)} ·{' '}
            {formatRelativeTime(fourDaysAgo)} · {pluralize(32, t.plural.draft)} ·{' '}
            {parseRankTier(51).label} · {parseRankTier(80, 123).label}
          </p>
        </Card>
      </Section>

      <Section title="Кнопки">
        <Card className="space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <Button>Купить разбор · 67 ₽</Button>
            <Button variant="accent2">Играть →</Button>
            <Button variant="surface">Купить</Button>
            <Button variant="outline">
              <MessageCircle size={18} /> Отзыв
            </Button>
            <Button variant="ghost">Сводка</Button>
            <Button variant="danger">Выйти</Button>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Button size="sm">Маленькая</Button>
            <Button size="md">Средняя</Button>
            <Button size="lg">Большая</Button>
            <Button size="xl">
              <SteamIcon size={22} /> Войти через Steam
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Button loading={loading} onClick={fakeLoad}>
              {loading ? 'Загружаем…' : 'Нажми — загрузка'}
            </Button>
            <Button variant="accent2" disabled>
              Скоро
            </Button>
            <IconButton aria-label="Обновить" variant="outline">
              <RotateCw size={20} />
            </IconButton>
            <IconButton aria-label="Поиск" variant="surface" size="icon-sm">
              <Search size={18} />
            </IconButton>
            <ButtonLink to="/" variant="outline">
              Ссылка‑кнопка
            </ButtonLink>
          </div>
        </Card>
      </Section>

      <Section title="Карточки">
        <div className="grid gap-3 md:grid-cols-3">
          <Card>
            <p className="text-card-title">Обычная</p>
            <p className="mt-1 text-[14px] text-muted">--surface, рамка, радиус 16</p>
          </Card>
          <Card interactive>
            <p className="text-card-title">С hover</p>
            <p className="mt-1 text-[14px] text-muted">Фон --surface-2 при наведении</p>
          </Card>
          <CardLink to="/play" className="flex items-center gap-3">
            <span className="flex size-12 items-center justify-center rounded-[12px] bg-surface-2">
              <Users size={22} />
            </span>
            <span>
              <span className="text-card-title block">Карточка‑ссылка</span>
              <span className="text-[14px] text-muted">Ведёт на /play</span>
            </span>
          </CardLink>
        </div>
      </Section>

      <Section title="Табы и переключатели">
        <Card className="space-y-5">
          <Tabs
            aria-label="Фильтр матчей"
            value={tab}
            onChange={setTab}
            items={[
              { value: 'all', label: 'Все', count: 19 },
              { value: 'win', label: 'Победы', count: 14 },
              { value: 'loss', label: 'Поражения', count: 5 },
            ]}
          />
          <Segmented
            aria-label="Режим драфтера"
            value={mode}
            onChange={setMode}
            items={[
              { value: 'analysis', label: 'Анализ', icon: Layers },
              { value: 'training', label: 'Тренировка', icon: Swords },
            ]}
          />
          <div className="flex items-center gap-4">
            {(['all', 'patch'] as const).map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => setPeriod(p)}
                className={
                  period === p ? 'font-medium text-fg' : 'font-medium text-muted hover:text-fg'
                }
              >
                {p === 'all' ? 'Все версии' : '7.41F'}
              </button>
            ))}
          </div>
          <div className="divide-y divide-line">
            <label className="flex items-center gap-4 py-3">
              <span className="flex-1">
                <span className="block font-medium">Микрофон</span>
                <span className="text-[14px] text-muted">Готов общаться голосом</span>
              </span>
              <Toggle checked={mic} onChange={setMic} label="Микрофон" />
            </label>
            <label className="flex items-center gap-4 py-3">
              <span className="flex-1">
                <span className="block font-medium">Discord</span>
                <span className="text-[14px] text-muted">Есть аккаунт</span>
              </span>
              <Toggle checked={discord} onChange={setDiscord} label="Discord" />
            </label>
          </div>
        </Card>
      </Section>

      <Section title="Чипы">
        <Card className="flex flex-wrap items-center gap-2">
          <Chip>Роумер</Chip>
          <Chip tone="accent">контроль</Chip>
          <Chip tone="win" size="sm">
            Победа
          </Chip>
          <Chip tone="loss" size="sm">
            Поражение
          </Chip>
          <Chip tone="outline">простой</Chip>
          {[
            ['ranked', 'Рейтинговый'],
            ['normal', 'Обычный'],
            ['turbo', 'Турбо'],
          ].map(([id, label]) => (
            <Chip key={id} selected={chips.includes(id)} onClick={() => toggleChip(id)}>
              {label}
            </Chip>
          ))}
        </Card>
      </Section>

      <Section title="Позиции">
        <Card className="grid gap-4 sm:grid-cols-5">
          {POSITIONS.map((p) => (
            <div key={p} className="flex flex-col items-center gap-2 text-center">
              <div className="flex size-[88px] items-center justify-center rounded-[12px] border border-line bg-bg">
                <PositionIcon position={p} size={40} />
              </div>
              <div className="flex items-center gap-1.5">
                <PositionIcon position={p} size={18} />
                <PositionIcon position={p} size={14} />
                <span className="text-[14px] font-medium">{positionShort(p)}</span>
              </div>
              <span className="text-[12px] text-muted">{positionFull(p)}</span>
            </div>
          ))}
        </Card>
      </Section>

      <Section title="Герои из /api/heroes">
        <HeroesDemo />
      </Section>

      <Section title="Прогресс, скелетоны, аватары">
        <div className="grid gap-3 md:grid-cols-3">
          <Card className="space-y-3">
            <div className="flex justify-between text-[14px] text-muted">
              <span>Вопрос 2 из 5</span>
              <span className="tabular-nums">{Math.round(progress * 100)}%</span>
            </div>
            <ProgressBar value={progress} label="Прогресс квиза" />
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="surface"
                onClick={() => setProgress((v) => Math.max(0, v - 0.2))}
              >
                {MINUS} 20%
              </Button>
              <Button
                size="sm"
                variant="surface"
                onClick={() => setProgress((v) => Math.min(1, v + 0.2))}
              >
                + 20%
              </Button>
            </div>
          </Card>
          <Card className="space-y-3">
            <div className="flex items-center gap-3">
              <Skeleton radius="full" className="size-12" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-2/3" />
                <Skeleton className="h-3 w-1/3" />
              </div>
            </div>
            <div className="flex gap-2">
              {[0, 1, 2, 3, 4].map((i) => (
                <Skeleton key={i} className="aspect-square flex-1" />
              ))}
            </div>
          </Card>
          <Card className="flex items-center gap-3">
            <Avatar name="Dendi" size={64} radius={14} />
            <Avatar name="puppey" size={48} />
            <Avatar src="https://invalid.example/avatar.png" name="Игрок" size={36} />
            <Avatar size={36} />
          </Card>
        </div>
      </Section>

      <Section title="Аккордеоны, подсказки, окно, тосты">
        <div className="grid gap-3 md:grid-cols-2">
          <Card className="divide-y divide-line py-1">
            <Accordion title="Уже включена?" icon="plus">
              Данные появляются с задержкой. Сыграй хотя бы один матч после включения настройки и
              нажми «Проверить снова». Если не помогло — подожди 15–30 минут.
            </Accordion>
            <Accordion title="Помощь с оплатой">Короткий FAQ по оплате.</Accordion>
          </Card>
          <Card className="flex flex-wrap items-center gap-3">
            <Tooltip content="Pudge · 2 / 11 / 23 · 4 дн. назад">
              <button
                type="button"
                aria-label="Матч"
                className="size-[14px] rounded-[4px] bg-win"
              />
            </Tooltip>
            <Tooltip content="Поражение">
              <button
                type="button"
                aria-label="Матч"
                className="size-[14px] rounded-[4px] bg-loss"
              />
            </Tooltip>
            <Button variant="surface" onClick={() => setSheetOpen(true)}>
              <Crosshair size={18} /> Открыть окно
            </Button>
            <Button variant="outline" onClick={() => toast.success('Анкета сохранена')}>
              <Bell size={18} /> Тост
            </Button>
            <Button variant="outline" onClick={() => toast.error(t.states.network)}>
              <Heart size={18} className="text-loss-strong" /> Ошибка
            </Button>
          </Card>
        </div>
      </Section>

      <Section title="Состояния">
        <div className="grid gap-3 md:grid-cols-2">
          <Card padding="none">
            <EmptyState
              icon={Search}
              title="Ничего не нашли"
              description="Попробуй изменить фильтры"
            />
          </Card>
          <Card padding="none">
            <ErrorState onRetry={fakeLoad} retrying={loading} />
          </Card>
        </div>
      </Section>

      <Sheet
        open={sheetOpen}
        onClose={() => setSheetOpen(false)}
        title="Выбор героя"
        width={880}
        height={640}
      >
        <input
          type="search"
          placeholder="Найти героя…"
          className="mb-4 h-11 w-full rounded-[12px] border border-line bg-bg px-4 outline-none placeholder:text-dim focus:border-accent-2"
        />
        <div className="grid grid-cols-4 gap-2 md:grid-cols-8">
          {Array.from({ length: 32 }, (_, i) => (
            <Skeleton key={i} className="aspect-video" />
          ))}
        </div>
      </Sheet>
    </Page>
  )
}

/** Портреты в размерах из SPEC: мета 132×132, сетка драфтера 120×68, список матчей 56×32, баны. */
function HeroesDemo() {
  const { data, isPending, isError, refetch } = useHeroes()
  const { data: patch } = usePatch()

  if (isPending) return <Skeleton className="h-[220px]" />
  if (isError)
    return (
      <Card padding="none">
        <ErrorState onRetry={() => refetch()} />
      </Card>
    )

  const sample = data.list.slice(0, 5)
  return (
    <Card className="space-y-5">
      <p className="text-[14px] text-muted">
        Героев: {data.list.length} · патч{' '}
        <span className="font-mono text-fg">{patch?.label ?? '…'}</span>
      </p>
      <div className="flex flex-wrap gap-2">
        {sample.map((hero) => (
          <div key={hero.id} className="w-[132px] text-center">
            <HeroPortrait hero={hero} width={132} height={132} />
            <p className="mt-1 truncate text-[13px] text-muted">{hero.localized_name}</p>
          </div>
        ))}
      </div>
      <div className="grid grid-cols-6 gap-2">
        {data.list.slice(5, 17).map((hero) => (
          <HeroPortrait key={hero.id} hero={hero} width={120} height={68} />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        {data.list.slice(17, 23).map((hero) => (
          <HeroPortrait key={hero.id} hero={hero} width={56} height={32} radius={8} />
        ))}
        {data.list.slice(23, 28).map((hero) => (
          <HeroPortrait key={hero.id} hero={hero} width={48} height={27} radius={6} grayscale />
        ))}
        {data.list.slice(28, 34).map((hero) => (
          <HeroIcon key={hero.id} hero={hero} width={28} height={28} />
        ))}
        <HeroPortrait hero={undefined} width={56} height={32} radius={8} />
      </div>
    </Card>
  )
}
