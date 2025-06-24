import { useEffect } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { Theme } from '@radix-ui/themes'
import '@radix-ui/themes/styles.css'

import Actions from './components/Actions/Actions'
import Chat from './components/Chat/Chat'
import FileViewer from './components/Files/Viewer'
import Login from './components/Login/Login'
import NotFound from './components/NotFound'
import Settings from './components/Settings/Settings'
import { AgentClientProvider } from './contexts/AgentContext'
import { BlockProvider } from './contexts/BlockContext'
import { SettingsProvider, useSettings } from './contexts/SettingsContext'
import { WebAppConfigJson } from './gen/es/cassie/config/webapp_pb'
import { Code } from './gen/es/google/rpc/code_pb'
import Layout from './layout'

export interface AppProps {
  initialState?: {
    requireAuth?: boolean
    webApp?: WebAppConfigJson
  }
  logo: string
}

function AppRouter({ logo }: AppProps) {
  const { settings, runnerError } = useSettings()

  useEffect(() => {
    if (!runnerError) {
      return
    }

    const settingsPath = '/settings'
    const currentPath = window.location.pathname
    if (
      currentPath === settingsPath ||
      currentPath === '/login' ||
      currentPath === '/oidc/login'
    ) {
      return
    }

    const loginUrl = settings.requireAuth ? '/oidc/login' : '/login'

    if (!(runnerError instanceof Error) && !(runnerError instanceof Event)) {
      const isAuthError =
        runnerError.code === Code.UNAUTHENTICATED ||
        runnerError.code === Code.PERMISSION_DENIED
      const redirectUrl = isAuthError ? loginUrl : settingsPath
      window.location.href = redirectUrl
      return
    }

    window.location.href = settingsPath
  }, [runnerError, settings.requireAuth])

  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={
            <Layout
              logo={logo}
              left={<Chat />}
              middle={<Actions />}
              right={<FileViewer />}
            />
          }
        />
        <Route
          path="/settings"
          element={
            <Layout
              logo={logo}
              left={<Chat />}
              middle={<Actions />}
              right={<Settings />}
            />
          }
        />
        <Route
          path="/oidc/*"
          element={
            <Layout
              logo={logo}
              middle={
                <div>OIDC routes are exclusively handled by the server.</div>
              }
            />
          }
        />
        <Route
          path="/login"
          element={<Layout logo={logo} left={<Login />} />}
        />
        <Route path="*" element={<Layout logo={logo} left={<NotFound />} />} />
      </Routes>
    </BrowserRouter>
  )
}

function App({ initialState = {}, logo }: AppProps) {
  return (
    <>
      <title>Cloud Assistant</title>
      <meta name="description" content="An AI Assistant For Your Cloud" />
      <link rel="icon" href={logo} />
      <Theme accentColor="gray" scaling="100%" radius="small">
        <SettingsProvider
          requireAuth={initialState?.requireAuth}
          webApp={initialState?.webApp}
        >
          <AgentClientProvider>
            <BlockProvider>
              <AppRouter logo={logo} />
            </BlockProvider>
          </AgentClientProvider>
        </SettingsProvider>
      </Theme>
    </>
  )
}

export default App
