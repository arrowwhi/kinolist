import { Component } from 'react'
import type { ErrorInfo, ReactNode } from 'react'
import { Button } from '../shared/ui/Button'
export class ErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false }
  static getDerivedStateFromError() {
    return { failed: true }
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Application render error', error, info.componentStack)
  }
  render() {
    if (!this.state.failed) return this.props.children
    return (
      <main className="app-shell stack">
        <h1>Не получилось открыть страницу</h1>
        <p className="muted">
          Перезагрузи приложение. Сохранённые фильмы останутся в коллекции.
        </p>
        <Button onClick={() => window.location.reload()}>Перезагрузить</Button>
      </main>
    )
  }
}
