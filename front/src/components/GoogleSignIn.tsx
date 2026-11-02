import { useEffect, useRef } from 'react'
import { config } from '@/lib/config'
import { loadGoogleIdentity } from '@/lib/google'

export function GoogleSignIn({
  mode,
  disabled,
  onCredential,
  onError,
}: {
  mode: 'login' | 'register'
  disabled?: boolean
  onCredential: (credential: string) => void | Promise<void>
  onError: (message: string) => void
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  const credentialRef = useRef(onCredential)
  const errorRef = useRef(onError)

  credentialRef.current = onCredential
  errorRef.current = onError

  useEffect(() => {
    if (!config.googleClientId) return

    let cancelled = false
    loadGoogleIdentity()
      .then(() => {
        const container = containerRef.current
        const accountsId = window.google?.accounts?.id
        if (cancelled || !container || !accountsId) return

        accountsId.initialize({
          client_id: config.googleClientId,
          ux_mode: 'popup',
          callback: (response) => {
            if (response.credential) {
              void credentialRef.current(response.credential)
            } else {
              errorRef.current('Google did not return a credential. Try again.')
            }
          },
        })

        container.replaceChildren()
        accountsId.renderButton(container, {
          type: 'standard',
          theme: 'filled_black',
          size: 'large',
          shape: 'rectangular',
          text: mode === 'register' ? 'signup_with' : 'signin_with',
          logo_alignment: 'left',
          width: Math.min(400, Math.max(220, container.clientWidth || 340)),
        })
      })
      .catch((error: unknown) => {
        if (cancelled) return
        errorRef.current(error instanceof Error ? error.message : 'Could not load Google sign-in.')
      })

    return () => {
      cancelled = true
    }
  }, [mode])

  if (!config.googleClientId) return null

  return (
    <div className={disabled ? 'google-signin is-disabled' : 'google-signin'} aria-busy={disabled}>
      <div ref={containerRef} className="google-signin__button" />
    </div>
  )
}
