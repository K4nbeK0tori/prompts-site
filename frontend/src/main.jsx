import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider, App as AntApp, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import Site from './App'
import { lightTheme, darkTheme } from './theme'
import { readPrefs, writePrefs } from './prefs'
import './styles.css'

dayjs.locale('zh-cn')

function Root() {
  const initial = React.useMemo(() => readPrefs(), [])
  const prefersDark =
    typeof window !== 'undefined' &&
    window.matchMedia &&
    window.matchMedia('(prefers-color-scheme: dark)').matches

  // 支持 ?theme=dark / ?theme=light 临时覆盖，方便分享与预览
  const urlTheme = React.useMemo(() => {
    try {
      return new URLSearchParams(window.location.search).get('theme')
    } catch {
      return null
    }
  }, [])

  const [dark, setDark] = React.useState(() => {
    if (urlTheme === 'dark') return true
    if (urlTheme === 'light') return false
    return typeof initial.dark === 'boolean' ? initial.dark : prefersDark
  })

  const toggleDark = React.useCallback((next) => {
    setDark(next)
    writePrefs({ dark: next })
  }, [])

  React.useEffect(() => {
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  }, [dark])

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        ...(dark ? darkTheme : lightTheme),
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
      }}
    >
      <AntApp>
        <Site dark={dark} onToggleDark={toggleDark} initialPrefs={initial} />
      </AntApp>
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
)
