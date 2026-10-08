const PREF_KEY = 'prompts-site-prefs'

export function readPrefs() {
  try {
    const raw = localStorage.getItem(PREF_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch {
    return {}
  }
}

export function writePrefs(patch) {
  try {
    const next = { ...readPrefs(), ...patch }
    localStorage.setItem(PREF_KEY, JSON.stringify(next))
    return next
  } catch {
    return patch
  }
}
