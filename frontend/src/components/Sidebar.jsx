import React from 'react'
import { Button, Segmented, Switch, Tooltip } from 'antd'
import { ClearOutlined, HeartFilled, TagsOutlined } from '@ant-design/icons'

export function SakuraLogo({ size = 38 }) {
  return (
    <svg className="brand-logo" width={size} height={size} viewBox="0 0 48 48" aria-hidden="true">
      <defs>
        <linearGradient id="sakuraGrad" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#fbcfe8" />
          <stop offset="55%" stopColor="#f472b6" />
          <stop offset="100%" stopColor="#ec4899" />
        </linearGradient>
      </defs>
      <g fill="url(#sakuraGrad)">
        {[0, 72, 144, 216, 288].map((deg) => (
          <path
            key={deg}
            d="M24 22 C 18 16, 18 7, 24 3 C 30 7, 30 16, 24 22 Z"
            transform={`rotate(${deg} 24 24)`}
          />
        ))}
      </g>
      <circle cx="24" cy="24" r="4.6" fill="#fff5f9" />
      <circle cx="24" cy="24" r="2" fill="#fbbf24" />
    </svg>
  )
}

export default function Sidebar({
  stats,
  tags,
  selectedTags,
  onToggleTag,
  source,
  onSource,
  status,
  onStatus,
  favoriteOnly,
  onFavoriteOnly,
  onReset,
  activeCount,
}) {
  const sourceOptions = [
    { label: '全部', value: '' },
    ...((stats?.sources || [])
      .filter((s) => s.tag)
      .map((s) => ({
        label: `${s.tag} ${s.count}`,
        value: s.tag,
      }))),
  ]

  return (
    <aside className="sidebar">
      <div className="panel">
        <div className="panel-title">
          <span>收藏概览</span>
        </div>
        <div className="stat-grid">
          <div className="stat-cell">
            <b>{stats?.total ?? '—'}</b>
            <span>总条数</span>
          </div>
          <div className="stat-cell">
            <b>{stats?.enabled ?? '—'}</b>
            <span>已启用</span>
          </div>
          <div className="stat-cell">
            <b>{stats?.disabled ?? '—'}</b>
            <span>已停用</span>
          </div>
          <div className="stat-cell">
            <b>{stats?.favorite ?? '—'}</b>
            <span>已收藏</span>
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-title">
          <span>
            <TagsOutlined /> 标签
          </span>
          <span style={{ letterSpacing: 0 }}>{tags.length}</span>
        </div>
        <div className="tag-cloud">
          {tags.map((t) => (
            <button
              type="button"
              key={t.tag}
              className={`tag-chip${selectedTags.includes(t.tag) ? ' on' : ''}`}
              onClick={() => onToggleTag(t.tag)}
            >
              {t.tag}
              <em>{t.count}</em>
            </button>
          ))}
          {tags.length === 0 && <span style={{ fontSize: 12, color: 'var(--ink-2)' }}>暂无标签</span>}
        </div>
      </div>

      <div className="panel">
        <div className="panel-title">
          <span>筛选</span>
          {activeCount > 0 && (
            <Tooltip title="清空全部筛选">
              <Button
                size="small"
                type="text"
                icon={<ClearOutlined />}
                onClick={onReset}
                style={{ fontSize: 12 }}
              >
                清空
              </Button>
            </Tooltip>
          )}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Segmented
            block
            size="small"
            value={source || ''}
            options={sourceOptions}
            onChange={onSource}
          />
          <Segmented
            block
            size="small"
            value={status || ''}
            options={[
              { label: '全部', value: '' },
              { label: '已启用', value: 'enabled' },
              { label: '已停用', value: 'disabled' },
            ]}
            onChange={onStatus}
          />
          <label
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              fontSize: 13,
            }}
          >
            <span>
              <HeartFilled style={{ color: '#f59e0b', marginRight: 6 }} />
              只看收藏
            </span>
            <Switch size="small" checked={favoriteOnly} onChange={onFavoriteOnly} />
          </label>
        </div>
      </div>
    </aside>
  )
}
