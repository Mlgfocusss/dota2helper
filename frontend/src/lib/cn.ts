import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Склеивает классы и убирает конфликтующие классы Tailwind (последний побеждает). */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
