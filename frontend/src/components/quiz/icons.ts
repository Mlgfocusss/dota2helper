import {
  Anchor,
  Coins,
  Crosshair,
  Crown,
  Flame,
  HandHeart,
  Move,
  Scale,
  Shield,
  Sword,
  Users,
  Zap,
  CircleHelp,
  type LucideIcon,
} from 'lucide-react'

// Иконки вариантов ответа: имена из quiz_position.json → компоненты lucide-react
const ICONS: Record<string, LucideIcon> = {
  coins: Coins,
  crosshair: Crosshair,
  sword: Sword,
  'hand-heart': HandHeart,
  move: Move,
  flame: Flame,
  users: Users,
  scale: Scale,
  zap: Zap,
  anchor: Anchor,
  crown: Crown,
  shield: Shield,
}

export function quizIcon(name: string): LucideIcon {
  return ICONS[name] ?? CircleHelp
}
