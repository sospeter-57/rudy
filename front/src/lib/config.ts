/**
 * Runtime configuration
 * ---------------------
 * Every tunable value the app uses comes from a Vite environment variable with
 * a safe default, so the same build can be re-pointed without touching code.
 * See `.env.example` for the full list.
 *
 * Vite only exposes variables prefixed with `VITE_` to the client. Values are
 * read once at module load.
 */

const env = import.meta.env as unknown as Record<string, string | undefined>

function str(key: string, fallback: string): string {
  const raw = env[key]
  return raw === undefined || raw.trim() === '' ? fallback : raw.trim()
}

function num(key: string, fallback: number): number {
  const raw = env[key]
  if (raw === undefined || raw.trim() === '') return fallback
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : fallback
}

function int(key: string, fallback: number): number {
  return Math.round(num(key, fallback))
}

function bool(key: string, fallback: boolean): boolean {
  const raw = env[key]
  if (raw === undefined || raw.trim() === '') return fallback
  return ['1', 'true', 'yes', 'on'].includes(raw.trim().toLowerCase())
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

const taskPointsMin = Math.max(0, int('VITE_TASK_POINTS_MIN', 1))
const taskPointsMax = Math.max(taskPointsMin + 1, int('VITE_TASK_POINTS_MAX', 100))

const journeymanMin = Math.max(1, int('VITE_TIER_JOURNEYMAN_MIN', 100))
const artisanMin = Math.max(journeymanMin + 1, int('VITE_TIER_ARTISAN_MIN', 250))
const masterMin = Math.max(artisanMin + 1, int('VITE_TIER_MASTER_MIN', 500))
const legendMin = Math.max(masterMin + 1, int('VITE_TIER_LEGEND_MIN', 1000))

export const config = {
  /** Brand name shown in the rail and on the landing page. */
  appName: str('VITE_APP_NAME', 'Rudy'),

  /** Base path/URL prepended to every HTTP API request. */
  apiBase: str('VITE_API_BASE', '/api'),
  /** Abort an HTTP request after this many milliseconds. */
  requestTimeoutMs: Math.max(1000, int('VITE_REQUEST_TIMEOUT_MS', 15_000)),

  /** OAuth 2.0 client ID for Google sign-in. Empty hides the Google button. */
  googleClientId: str('VITE_GOOGLE_CLIENT_ID', ''),

  /** Show the one-click demo accounts on the landing page. */
  demoLogin: bool('VITE_ENABLE_DEMO_LOGIN', true),
  /** Password shared by the seeded demo accounts. */
  demoPassword: str('VITE_DEMO_PASSWORD', 'password123'),

  /** Reviewer bonus as a fraction of the task's points (0–1). */
  reviewRate: clamp(num('VITE_REVIEW_RATE', 0.15), 0, 1),
  /** Floor for a review bonus so tiny tasks still pay reviewers. */
  minReviewBonus: Math.max(0, int('VITE_MIN_REVIEW_BONUS', 2)),

  taskPointsMin,
  taskPointsMax,
  defaultTaskPoints: clamp(int('VITE_DEFAULT_TASK_POINTS', 20), taskPointsMin, taskPointsMax),

  projectNameMin: Math.max(1, int('VITE_PROJECT_NAME_MIN', 3)),
  projectNameMax: Math.max(3, int('VITE_PROJECT_NAME_MAX', 24)),
  projectTtlMin: Math.max(1, int('VITE_PROJECT_TTL_MIN', 30)),
  projectTtlDefault: Math.max(1, int('VITE_PROJECT_TTL_DEFAULT', 1440)),

  usernameMin: Math.max(1, int('VITE_USERNAME_MIN', 3)),
  usernameMax: Math.max(3, int('VITE_USERNAME_MAX', 24)),
  passwordMin: Math.max(1, int('VITE_PASSWORD_MIN', 8)),

  /** Tier thresholds. Names, tones and blurbs live in `points.ts`. */
  tierThresholds: {
    apprentice: 0,
    journeyman: journeymanMin,
    artisan: artisanMin,
    master: masterMin,
    legend: legendMin,
  },

  queryStaleTimeMs: Math.max(0, int('VITE_QUERY_STALE_TIME_MS', 10_000)),
  meStaleTimeMs: Math.max(0, int('VITE_ME_STALE_TIME_MS', 5_000)),
  queryRetry: Math.max(0, int('VITE_QUERY_RETRY', 1)),
  toastDurationMs: Math.max(500, int('VITE_TOAST_DURATION_MS', 4600)),
} as const

export type AppConfig = typeof config
