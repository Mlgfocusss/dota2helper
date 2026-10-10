import { describe, expect, it } from 'vitest'
import { joinLabels, MAX_CHOICES, optionIndexFromKey, toggleChoice } from './quiz'

describe('toggleChoice', () => {
  it('добавляет варианты до двух', () => {
    expect(toggleChoice([], 'a')).toEqual(['a'])
    expect(toggleChoice(['a'], 'b')).toEqual(['a', 'b'])
  })

  it('третий вариант выбрать нельзя', () => {
    expect(toggleChoice(['a', 'b'], 'c')).toEqual(['a', 'b'])
    expect(MAX_CHOICES).toBe(2)
  })

  it('повторный клик снимает выбор, в том числе при двух выбранных', () => {
    expect(toggleChoice(['a'], 'a')).toEqual([])
    expect(toggleChoice(['a', 'b'], 'a')).toEqual(['b'])
  })

  it('не меняет исходный массив', () => {
    const src = ['a', 'b']
    toggleChoice(src, 'c')
    toggleChoice(src, 'a')
    expect(src).toEqual(['a', 'b'])
  })
})

describe('optionIndexFromKey', () => {
  it('переводит клавиши 1–4 в индексы вариантов', () => {
    expect(optionIndexFromKey('1', 4)).toBe(0)
    expect(optionIndexFromKey('4', 4)).toBe(3)
  })

  it('игнорирует лишние клавиши и номера больше числа вариантов', () => {
    expect(optionIndexFromKey('5', 4)).toBeNull()
    expect(optionIndexFromKey('0', 4)).toBeNull()
    expect(optionIndexFromKey('a', 4)).toBeNull()
    expect(optionIndexFromKey('Enter', 4)).toBeNull()
    expect(optionIndexFromKey('3', 2)).toBeNull()
  })
})

describe('joinLabels', () => {
  it('склеивает перечисление с «и» перед последним', () => {
    expect(joinLabels([])).toBe('')
    expect(joinLabels(['контроль'])).toBe('контроль')
    expect(joinLabels(['контроль', 'сейвы'])).toBe('контроль и сейвы')
    expect(joinLabels(['контроль', 'сейвы', 'лечение'])).toBe('контроль, сейвы и лечение')
  })
})
