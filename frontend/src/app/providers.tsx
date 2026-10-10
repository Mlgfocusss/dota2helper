import { QueryClientProvider } from '@tanstack/react-query'
import { MotionConfig } from 'framer-motion'
import type { ReactNode } from 'react'
import { queryClient } from './queryClient'

export function Providers({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <MotionConfig reducedMotion="user">{children}</MotionConfig>
    </QueryClientProvider>
  )
}
