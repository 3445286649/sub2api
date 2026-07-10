type LocaleObject = Record<string, unknown>

function isLocaleObject(value: unknown): value is LocaleObject {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

export function mergeLocaleFallback<T extends LocaleObject>(
  current: T,
  fallback: LocaleObject,
): T {
  const merged: LocaleObject = { ...current }

  for (const [key, fallbackValue] of Object.entries(fallback)) {
    const currentValue = merged[key]
    if (currentValue === undefined) {
      merged[key] = fallbackValue
    } else if (isLocaleObject(currentValue) && isLocaleObject(fallbackValue)) {
      merged[key] = mergeLocaleFallback(currentValue, fallbackValue)
    }
  }

  return merged as T
}
