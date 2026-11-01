import { Link } from 'react-router-dom'
import { Compass } from 'lucide-react'

export function NotFoundPage() {
  return (
    <div className="not-found">
      <span className="eyebrow">404</span>
      <h1>Off the map.</h1>
      <p>This page does not exist — or it moved with the last deploy.</p>
      <Link to="/dashboard" className="btn btn--primary btn--md">
        <Compass size={15} /> Back to the deck
      </Link>
    </div>
  )
}
