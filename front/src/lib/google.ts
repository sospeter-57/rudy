/**
 * Minimal wrapper around Google Identity Services.
 *
 * Native `<script>` loading is used instead of a bundler import so the client
 * only fetches Google's code when a client ID is configured. The fetched ID
 * token is verified by the Go API, which keeps the client-secret-free GIS flow
 * safe: the browser never decides who the user is.
 */

export interface GoogleCredentialResponse {
  credential: string
  select_by?: string
}

export interface GoogleIdConfiguration {
  client_id: string
  callback: (response: GoogleCredentialResponse) => void
  ux_mode?: 'popup' | 'redirect'
  auto_select?: boolean
  cancel_on_tap_outside?: boolean
}

export interface GoogleButtonOptions {
  type?: 'standard' | 'icon'
  theme?: 'outline' | 'filled_blue' | 'filled_black'
  size?: 'large' | 'medium' | 'small'
  text?: 'signin_with' | 'signup_with' | 'continue_with' | 'signin'
  shape?: 'rectangular' | 'pill' | 'circle' | 'square'
  logo_alignment?: 'left' | 'center'
  width?: number
}

export interface GoogleAccountsId {
  initialize: (configuration: GoogleIdConfiguration) => void
  renderButton: (parent: HTMLElement, options: GoogleButtonOptions) => void
  disableAutoSelect: () => void
}

declare global {
  interface Window {
    google?: {
      accounts?: {
        id: GoogleAccountsId
      }
    }
  }
}

let loader: Promise<void> | null = null
const scriptId = 'rudy-google-identity'

export function loadGoogleIdentity(): Promise<void> {
  if (typeof window === 'undefined') {
    return Promise.reject(new Error('Google sign-in needs a browser.'))
  }
  if (window.google?.accounts?.id) {
    return Promise.resolve()
  }
  if (loader) {
    return loader
  }

  loader = new Promise<void>((resolve, reject) => {
    const existing = document.getElementById(scriptId) as HTMLScriptElement | null
    const script = existing ?? document.createElement('script')

    script.addEventListener(
      'load',
      () => {
        if (window.google?.accounts?.id) resolve()
        else reject(new Error('Google sign-in did not initialise.'))
      },
      { once: true },
    )
    script.addEventListener('error', () => reject(new Error('Could not load Google sign-in.')), { once: true })

    if (!existing) {
      script.id = scriptId
      script.src = 'https://accounts.google.com/gsi/client'
      script.async = true
      script.defer = true
      document.head.appendChild(script)
    }
  }).catch((error: unknown) => {
    loader = null
    throw error
  })

  return loader
}
