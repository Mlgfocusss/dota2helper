// Типы ответов API (SPEC 11). Дополняются по мере реализации фич.
import type { Position } from '@/lib/positions'

export type { Position }

export type ApiErrorBody = {
  error: { code: string; message: string; fields?: Record<string, string> }
}

export type AppConfig = {
  app_name: string
  support_url: string
  support_contact: string
  promo: { image_url: string; link: string } | null
  /** deep_analysis — разбор включен и модель подключена */
  features: { deep_analysis: boolean; analyses_per_day: number }
}

export type Hero = {
  id: number
  name: string
  localized_name: string
  short_name: string
  primary_attr: 'str' | 'agi' | 'int' | 'all'
  roles: string[]
  img: string
  icon: string
}

export type Me = {
  user: {
    id: number
    account_id: number
    persona_name: string
    avatar: string
  } | null
  tracked_account_id: number | null
}

export type Patch = { number: string; label: string }

export type MetaHero = { hero_id: number; winrate: number; matches: number }

export type MetaPositions = {
  patch: Patch
  /** false — STRATZ недоступен: общий винрейт героя на его позициях */
  by_position: boolean
  updated_at: string
  positions: { position: Position; heroes: MetaHero[] }[]
}

export type NewsListItem = {
  gid: string
  title: string
  date: string
  url: string
  excerpt: string
}

/** html уже очищен санитайзером на бэке */
export type NewsItem = {
  gid: string
  title: string
  date: string
  url: string
  html: string
  /** Номер патча (7.41f), если новость о патче */
  patch?: string
}

export type PatchNote = { indent: number; text: string }
export type PatchSection = { title: string; notes: PatchNote[] }
export type PatchHero = {
  hero_id: number
  notes: PatchNote[]
  abilities: PatchSection[]
  talents: PatchNote[]
}
export type PatchNotes = {
  number: string
  date: string
  general: PatchSection[]
  items: PatchSection[]
  neutral_items: PatchSection[]
  heroes: PatchHero[]
}

export type NewsComment = {
  id: number
  text: string
  created_at: string
  author: string
  avatar: string
  /** Комментарий текущего пользователя — его можно удалить */
  mine: boolean
}
export type CommentsPage = { items: NewsComment[]; total: number; next_before: number | null }

export type DraftSlot = { hero_id: number; position: Position }

/** Матрица матчапов для расчёта «Анализа» на клиенте (GET /drafter/matrix). */
export type DraftMatrixPayload = {
  ids: number[]
  /** vs[i][j] — насколько winrate героя ids[i] против ids[j] выше ожидаемого, п. п. */
  vs: number[][]
  /** Синергия в одной команде; null — STRATZ недоступен */
  with: number[][] | null
  /** [позиция − 1] → герои, которых реально играют на позиции */
  playable: number[][]
  has_with: boolean
  updated_at: string
}

export type DraftPair = { a: number; b: number; value: number }

export type TeamAnalysis = { total: number; heroes: { hero_id: number; score: number }[] }

export type DraftAnalysis = {
  radiant: TeamAnalysis
  dire: TeamAnalysis
  best_pair: DraftPair | null
  best_matchup: DraftPair | null
  worst_matchup: DraftPair | null
}

export type TrainingStart = { training_id: string; enemies: DraftSlot[] }

export type TrainingBreakdown = {
  /** Вклад слагаемых в счёт, в очках */
  contrib: { vs: number; synergy: number; meta: number; penalty: number }
  off_position: number
  heroes: {
    hero_id: number
    position: Position
    vs: number
    synergy: number
    meta: number
    off_position: boolean
  }[]
  best_matchup: DraftPair | null
  worst_matchup: DraftPair | null
  best_pair: DraftPair | null
}

export type TrainingResult = {
  score: number
  breakdown: TrainingBreakdown
  month_rank: number | null
}

export type TrainingHistoryItem = {
  id: string
  submitted_at: string
  score: number
  team: DraftSlot[]
  enemies: DraftSlot[]
  breakdown: TrainingBreakdown | null
}

export type TrainingHistoryPage = { items: TrainingHistoryItem[]; next_cursor: string | null }

export type LeaderboardRow = {
  rank: number
  user_id: number
  name: string
  avatar: string | null
  drafts: number
  score: number
}

export type AnalysisStatus = 'queued' | 'parsing' | 'analyzing' | 'done' | 'failed'

export type MatchRow = {
  match_id: number
  hero_id: number
  win: boolean
  kills: number
  deaths: number
  assists: number
  duration: number
  start_time: string
  game_mode: number
  analysis?: { id: string; status: AnalysisStatus }
}

export type CoachSummary = {
  player: {
    account_id: number
    persona_name: string | null
    avatar: string | null
    rank_tier: number | null
    leaderboard_rank: number | null
  }
  has_history: boolean
  /** results — от старых к новым */
  last20: { wins: number; losses: number; results: ('W' | 'L')[] }
  six_months: { winrate: number; matches: number }
  heroes_in_form: {
    period: 'all' | string
    heroes: { hero_id: number; winrate: number; matches: number }[]
  }[]
  last_match: MatchRow | null
  patch: Patch
  /** Сколько матчей игрока сохранено у нас */
  matches_stored: number
}

export type AnalysisResult = {
  summary: { verdict: string; rating: number; key_stats: { label: string; value: string }[] }
  lanes: { text: string; lh_10: number; dn_10: number; gold_diff_10: number }
  episodes: {
    time: string
    title: string
    what_happened: string
    mistake: string | null
    better: string
    severity: 'good' | 'minor' | 'major'
  }[]
  improvements: { title: string; why: string; drill: string }[]
  next_games_plan: string[]
  /** Где игрок умирал в драках: координаты 0..1 для схематичной мини-карты */
  death_points: { x: number; y: number; n: number }[]
}

export type AnalysisView = {
  id: string
  match_id?: number
  status: AnalysisStatus
  /** Текст ошибки для пользователя (status = failed) */
  error?: string
  result?: AnalysisResult
  created_at?: string
  updated_at?: string
}

/** Строка списка «Разборы»: сведения о матче и всё нужное для «Что тренировать» */
export type AnalysisListItem = {
  id: string
  match_id: number
  status: AnalysisStatus
  error?: string
  created_at: string
  hero_id?: number
  win?: boolean
  kills?: number
  deaths?: number
  assists?: number
  start_time?: string
  rating?: number
  improvements?: { title: string; why: string; drill: string }[]
  next_games_plan?: string[]
}

export type AnalysisQuota = { per_day: number; left: number; reset_at: string | null }

export type CoachPlayer = {
  account_id: number
  persona_name: string | null
  avatar: string | null
  rank_tier: number | null
  leaderboard_rank: number | null
}

export type FormHero = { hero_id: number; winrate: number; matches: number }

export type MatchCounts = { all: number; win: number; loss: number }
export type MatchesPage = { items: MatchRow[]; counts: MatchCounts; next_cursor: string | null }

export type ResolvedProfile = {
  account_id: number
  persona_name: string | null
  avatar: string | null
}

export type ItemRef = { key: string; name: string }

export type MatchPlayerStat = {
  account_id: number | null
  name: string
  hero_id: number
  team: 'radiant' | 'dire'
  level: number
  kills: number
  deaths: number
  assists: number
  last_hits: number
  denies: number
  gpm: number
  xpm: number
  net_worth: number
  hero_damage: number
  tower_damage: number
  healing: number
  /** 6 слотов, null — пусто */
  items: (ItemRef | null)[]
  neutral: ItemRef | null
  is_me: boolean
}

export type MatchDetail = {
  match_id: number
  radiant_win: boolean
  /** Результат просматриваемого игрока; null — его нет в матче */
  win: boolean | null
  duration: number
  start_time: string
  game_mode: number
  lobby_type: number
  radiant_score: number
  dire_score: number
  players: MatchPlayerStat[]
}

export type HeroValue = { hero_id: number; value: number }
export type HigherLowerMode = 'popularity' | 'kills' | 'deaths'
export type GameRecords = { higher_lower: Record<HigherLowerMode, number> }

export type PositionQuizOption = { id: string; icon: string; text: string }
export type PositionQuizData = {
  id: string
  questions: { id: string; text: string; options: PositionQuizOption[] }[]
}

export type MetaHeroPick = { hero_id: number; winrate: number; difficulty?: 'easy' | 'hard' }
export type PositionResult = {
  primary: Position
  secondary: Position | null
  /** Доля очков по позициям 1–5, в процентах */
  shares: [number, number, number, number, number]
  description: string
  heroes: MetaHeroPick[]
  saved: boolean
}
export type LastPosition = { position: Position | null }

export type HeroQuizQuestion = {
  id: string
  text: string
  options: { id: string; text: string }[]
}
export type HeroQuizData = { id: string; position: Position; questions: HeroQuizQuestion[] }
export type HeroPick = {
  hero_id: number
  score: number
  winrate: number | null
  complexity: 1 | 2 | 3
  matched: string[]
}
export type HeroResult = { position: Position; heroes: HeroPick[]; saved: boolean }

// ----- поиск пати и профиль (SPEC 4.4–4.6) -------------------------------------

export type PartyMode = 'ranked' | 'normal' | 'turbo'
export type PartyGoal = 'rating' | 'fun' | 'stack' | 'scrim'
/** team — автор ищет команду; players — набирает игроков в свою */
export type PartyLooking = 'team' | 'players'
export type PartyReportReason = 'spam' | 'abuse' | 'fake'

export type PartyCard = {
  user_id: number
  account_id: number
  persona_name: string
  avatar: string
  /** 0 — калибровка, 1–8 — Рекрут…Титан */
  rank_medal: number
  hours: number
  positions: Position[]
  modes: PartyMode[]
  goal: PartyGoal
  looking_for: PartyLooking
  mic: boolean
  discord: boolean
  discord_tag?: string
  telegram?: string
  fav_heroes: number[]
  about: string
  is_active: boolean
  bumped_at: string
  /** Только в «моей» анкете: когда можно поднять снова */
  next_bump_at?: string
}

/** Поля, которые пользователь заполняет в анкете. */
export type PartyCardInput = {
  rank_medal: number
  hours: number
  positions: Position[]
  modes: PartyMode[]
  goal: PartyGoal
  looking_for: PartyLooking
  mic: boolean
  discord: boolean
  discord_tag: string
  telegram: string
  fav_heroes: number[]
  about: string
}

export type PartyMe = {
  card: PartyCard | null
  /** Медаль из ранга OpenDota — подсказка для формы; null, если ранга нет */
  suggested_rank: number | null
}

export type PartyPage = { items: PartyCard[]; next_cursor: string | null }

export type ProfileStats = {
  higher_lower: Record<HigherLowerMode, number>
  quiz: {
    position: Position | null
    hero: { position: Position; hero_ids: number[] } | null
  }
  drafter: { trainings: number; best_score: number | null; month_rank: number | null }
}

export type LeaderboardData = {
  /** «2026-10» */
  month: string
  items: LeaderboardRow[]
  me: LeaderboardRow | null
}

export type HeroPageMatchup = { hero_id: number; value: number }

export type HeroPageData = {
  hero_id: number
  /** 1–3 */
  difficulty: number
  /** false — STRATZ недоступен: винрейт общий, на основной позиции героя */
  by_position: boolean
  positions: { position: Position; matches: number; winrate: number }[]
  strong_against: HeroPageMatchup[]
  weak_against: HeroPageMatchup[]
  allies: HeroPageMatchup[]
  items: Record<'start' | 'early' | 'mid' | 'late', ItemRef[]>
}
