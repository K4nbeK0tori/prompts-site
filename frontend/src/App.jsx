import React from 'react'
import {
  App as AntApp,
  Button,
  Drawer,
  Empty,
  Input,
  Pagination,
  Segmented,
  Select,
  Switch,
  Tooltip,
} from 'antd'
import {
  FilterOutlined,
  LoginOutlined,
  LogoutOutlined,
  MoonOutlined,
  PlusOutlined,
  SearchOutlined,
  SunOutlined,
  ClearOutlined,
} from '@ant-design/icons'
import { api } from './api'
import { writePrefs } from './prefs'
import SakuraLayer from './components/SakuraLayer'
import Sidebar, { SakuraLogo } from './components/Sidebar'
import PromptCard from './components/PromptCard'
import PromptDetail from './components/PromptDetail'
import PromptForm from './components/PromptForm'
import LoginModal from './components/LoginModal'

const SORT_OPTIONS = [
  { label: '默认排序', value: 'default' },
  { label: '最近更新', value: 'updated' },
  { label: '创建时间', value: 'created' },
  { label: '名称', value: 'title' },
  { label: '最早在前', value: 'oldest' },
]

const PAGE_SIZES = [12, 24, 48, 96]

export default function Site({ dark, onToggleDark, initialPrefs }) {
  const { message } = AntApp.useApp()

  const [prompts, setPrompts] = React.useState([])
  const [total, setTotal] = React.useState(0)
  const [pages, setPages] = React.useState(0)
  const [loading, setLoading] = React.useState(true)

  const [stats, setStats] = React.useState(null)
  const [tags, setTags] = React.useState([])

  const [query, setQuery] = React.useState('')
  const [searchInput, setSearchInput] = React.useState('')
  const [selectedTags, setSelectedTags] = React.useState([])
  const [source, setSource] = React.useState('')
  const [status, setStatus] = React.useState('')
  const [favoriteOnly, setFavoriteOnly] = React.useState(false)
  const [sort, setSort] = React.useState('default')
  const [page, setPage] = React.useState(1)
  const [size, setSize] = React.useState(24)

  const [isAdmin, setIsAdmin] = React.useState(false)
  const [loginOpen, setLoginOpen] = React.useState(false)
  const [loginLoading, setLoginLoading] = React.useState(false)
  const [loginError, setLoginError] = React.useState('')

  const [detail, setDetail] = React.useState(null)
  const [formOpen, setFormOpen] = React.useState(false)
  const [editing, setEditing] = React.useState(null)
  const [saving, setSaving] = React.useState(false)

  const [blurNSFW, setBlurNSFW] = React.useState(initialPrefs?.blurNSFW !== false)
  const [sakura, setSakura] = React.useState(initialPrefs?.sakura !== false)
  const [revealed, setRevealed] = React.useState(() => new Set())
  const [mobileFilterOpen, setMobileFilterOpen] = React.useState(false)

  const searchRef = React.useRef(null)

  // 搜索防抖
  React.useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(searchInput.trim())
      setPage(1)
    }, 320)
    return () => clearTimeout(timer)
  }, [searchInput])

  const loadMeta = React.useCallback(async () => {
    try {
      const [me, st, tg] = await Promise.all([api.me(), api.stats(), api.tags()])
      setIsAdmin(Boolean(me.authenticated))
      setStats(st)
      setTags(tg.tags || [])
    } catch (err) {
      message.error(err.message)
    }
  }, [message])

  const loadList = React.useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.list({
        q: query,
        tag: selectedTags,
        source,
        status,
        favorite: favoriteOnly ? 1 : '',
        sort,
        page,
        size,
      })
      setPrompts(data.items || [])
      setTotal(data.total || 0)
      setPages(data.pages || 0)
    } catch (err) {
      message.error(err.message)
      setPrompts([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }, [query, selectedTags, source, status, favoriteOnly, sort, page, size, message])

  React.useEffect(() => {
    loadMeta()
  }, [loadMeta])

  React.useEffect(() => {
    loadList()
  }, [loadList])

  // 快捷键：/ 聚焦搜索，Esc 关闭抽屉
  React.useEffect(() => {
    const onKey = (e) => {
      const tag = (e.target && e.target.tagName) || ''
      const typing = tag === 'INPUT' || tag === 'TEXTAREA' || (e.target && e.target.isContentEditable)
      if (e.key === '/' && !typing) {
        e.preventDefault()
        searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const activeCount =
    selectedTags.length + (source ? 1 : 0) + (status ? 1 : 0) + (favoriteOnly ? 1 : 0) + (query ? 1 : 0)

  const resetFilters = () => {
    setSelectedTags([])
    setSource('')
    setStatus('')
    setFavoriteOnly(false)
    setSearchInput('')
    setQuery('')
    setPage(1)
  }

  const toggleTag = (tag) => {
    setSelectedTags((prev) => (prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]))
    setPage(1)
  }

  const copyPrompt = async (prompt) => {
    try {
      await navigator.clipboard.writeText(prompt.content || '')
      message.success(`已复制「${prompt.title || '未命名'}」`)
    } catch {
      message.error('复制失败，请手动选择文本')
    }
  }

  const reveal = (id) => {
    setRevealed((prev) => {
      const next = new Set(prev)
      next.add(id)
      return next
    })
  }

  const toggleFavorite = async (prompt) => {
    try {
      const updated = await api.favorite(prompt.id, !prompt.favorite)
      setPrompts((prev) => prev.map((p) => (p.id === updated.id ? updated : p)))
      if (detail && detail.id === updated.id) setDetail(updated)
      loadMeta()
    } catch (err) {
      message.error(err.message)
    }
  }

  const handleLogin = async (password) => {
    setLoginLoading(true)
    setLoginError('')
    try {
      await api.login(password)
      setLoginOpen(false)
      await loadMeta()
      message.success('登录成功，现在可以编辑了')
    } catch (err) {
      setLoginError(err.message)
    } finally {
      setLoginLoading(false)
    }
  }

  const handleLogout = async () => {
    try {
      await api.logout()
      setIsAdmin(false)
      message.success('已退出登录')
    } catch (err) {
      message.error(err.message)
    }
  }

  const handleSubmit = async (values) => {
    setSaving(true)
    try {
      if (editing && editing.id) {
        await api.update(editing.id, values)
        message.success('已保存')
      } else {
        await api.create(values)
        message.success('已创建')
      }
      setFormOpen(false)
      setEditing(null)
      setDetail(null)
      await Promise.all([loadList(), loadMeta()])
    } catch (err) {
      message.error(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (prompt) => {
    try {
      await api.remove(prompt.id)
      message.success('已删除')
      setDetail(null)
      await Promise.all([loadList(), loadMeta()])
    } catch (err) {
      message.error(err.message)
    }
  }

  const openCreate = () => {
    setEditing(null)
    setFormOpen(true)
  }

  const openEdit = (prompt) => {
    setEditing(prompt)
    setFormOpen(true)
  }

  React.useEffect(() => {
    writePrefs({ blurNSFW, sakura })
  }, [blurNSFW, sakura])

  const filterPanel = (
    <Sidebar
      stats={stats}
      tags={tags}
      selectedTags={selectedTags}
      onToggleTag={toggleTag}
      source={source}
      onSource={(v) => {
        setSource(v || '')
        setPage(1)
      }}
      status={status}
      onStatus={(v) => {
        setStatus(v || '')
        setPage(1)
      }}
      favoriteOnly={favoriteOnly}
      onFavoriteOnly={(v) => {
        setFavoriteOnly(v)
        setPage(1)
      }}
      onReset={resetFilters}
      activeCount={activeCount}
    />
  )

  return (
    <div className="site-shell">
      <SakuraLayer enabled={sakura} />

      <header className="site-header">
        <div className="brand">
          <SakuraLogo />
          <div>
            <div className="brand-title">焚绝预设</div>
            <div className="brand-sub">AI 生图提示词收藏</div>
          </div>
        </div>

        <div className="header-spacer" />

        <Input
          ref={searchRef}
          allowClear
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          onPressEnter={() => {
            setQuery(searchInput.trim())
            setPage(1)
          }}
          prefix={<SearchOutlined style={{ color: 'var(--accent)' }} />}
          placeholder="搜索标题或提示词内容…  （按 / 快速聚焦）"
          style={{ maxWidth: 380, minWidth: 160 }}
        />

        <div className="header-actions">
          <Button
            className="mobile-filter-btn"
            icon={<FilterOutlined />}
            onClick={() => setMobileFilterOpen(true)}
          >
            筛选{activeCount > 0 ? ` (${activeCount})` : ''}
          </Button>

          <Tooltip title={blurNSFW ? '已遮罩 NSFW 内容' : 'NSFW 内容直接显示'}>
            <Button
              type={blurNSFW ? 'primary' : 'default'}
              ghost={blurNSFW}
              onClick={() => setBlurNSFW((v) => !v)}
              style={{ fontSize: 12 }}
            >
              NSFW 遮罩
            </Button>
          </Tooltip>

          <Tooltip title={sakura ? '关闭樱花飘落' : '开启樱花飘落'}>
            <Switch
              checked={sakura}
              onChange={setSakura}
              checkedChildren="🌸"
              unCheckedChildren="🌸"
            />
          </Tooltip>

          <Tooltip title={dark ? '切换到亮色' : '切换到暗色'}>
            <Button
              type="text"
              icon={dark ? <SunOutlined /> : <MoonOutlined />}
              onClick={() => onToggleDark(!dark)}
            />
          </Tooltip>

          {isAdmin ? (
            <>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                新建
              </Button>
              <Tooltip title="退出登录">
                <Button icon={<LogoutOutlined />} onClick={handleLogout} />
              </Tooltip>
            </>
          ) : (
            <Button icon={<LoginOutlined />} onClick={() => setLoginOpen(true)}>
              登录
            </Button>
          )}
        </div>
      </header>

      <div className="site-body">
        {filterPanel}

        <main className="main-area">
          <div className="toolbar">
            <span className="result-count">
              共 <b>{total}</b> 条
              {activeCount > 0 ? '（已筛选）' : ''}
            </span>
            <div className="header-spacer" />
            {activeCount > 0 && (
              <Button size="small" type="text" icon={<ClearOutlined />} onClick={resetFilters}>
                清空筛选
              </Button>
            )}
            <Select
              size="small"
              value={sort}
              options={SORT_OPTIONS}
              onChange={(v) => {
                setSort(v)
                setPage(1)
              }}
              style={{ width: 118 }}
            />
            <Segmented
              size="small"
              value={size}
              options={PAGE_SIZES.map((n) => ({ label: `${n}/页`, value: n }))}
              onChange={(v) => {
                setSize(v)
                setPage(1)
              }}
            />
          </div>

          {loading ? (
            <div className="prompt-grid">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="prompt-card" style={{ minHeight: 236 }}>
                  <div style={{ height: 14, width: '62%', borderRadius: 6, background: 'var(--line)' }} />
                  <div style={{ height: 10, width: '38%', borderRadius: 6, background: 'var(--line)' }} />
                  <div style={{ height: 96, borderRadius: 12, background: 'var(--line)' }} />
                </div>
              ))}
            </div>
          ) : prompts.length === 0 ? (
            <div className="empty-wrap">
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  activeCount > 0 ? '没有符合条件的提示词，换个筛选条件试试' : '还没有提示词'
                }
              />
            </div>
          ) : (
            <div className="prompt-grid">
              {prompts.map((p) => (
                <PromptCard
                  key={p.id}
                  prompt={p}
                  isAdmin={isAdmin}
                  blurNSFW={blurNSFW}
                  revealed={revealed.has(p.id)}
                  onReveal={reveal}
                  onOpen={setDetail}
                  onCopy={copyPrompt}
                  onFavorite={toggleFavorite}
                  onEdit={openEdit}
                  onDelete={handleDelete}
                />
              ))}
            </div>
          )}

          {pages > 1 && (
            <div className="pager">
              <Pagination
                current={page}
                pageSize={size}
                total={total}
                showSizeChanger={false}
                onChange={(p) => {
                  setPage(p)
                  window.scrollTo({ top: 0, behavior: 'smooth' })
                }}
              />
            </div>
          )}
        </main>
      </div>

      <Drawer
        open={mobileFilterOpen}
        onClose={() => setMobileFilterOpen(false)}
        title="筛选"
        placement="left"
        width={286}
      >
        <div style={{ margin: '-8px -8px 0' }}>{filterPanel}</div>
      </Drawer>

      <PromptDetail
        prompt={detail}
        open={Boolean(detail)}
        onClose={() => setDetail(null)}
        isAdmin={isAdmin}
        onEdit={openEdit}
        onDelete={handleDelete}
        onFavorite={toggleFavorite}
      />

      <PromptForm
        open={formOpen}
        prompt={editing}
        saving={saving}
        onCancel={() => {
          setFormOpen(false)
          setEditing(null)
        }}
        onSubmit={handleSubmit}
      />

      <LoginModal
        open={loginOpen}
        loading={loginLoading}
        error={loginError}
        onCancel={() => {
          setLoginOpen(false)
          setLoginError('')
        }}
        onSubmit={handleLogin}
      />
    </div>
  )
}
