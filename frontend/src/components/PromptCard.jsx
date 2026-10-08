import React from 'react'
import { Popconfirm, Tooltip } from 'antd'
import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  StarFilled,
  StarOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'

export function isNSFW(tags = []) {
  const set = ['nsfw', 'nfsw', 'r18', 'nsfl', '18+', 'adult']
  return tags.some((t) => set.includes(String(t).trim().toLowerCase()))
}

export function formatTime(value) {
  if (!value) return '—'
  const d = dayjs(value)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm') : value
}

export default function PromptCard({
  prompt,
  isAdmin,
  blurNSFW,
  onOpen,
  onCopy,
  onFavorite,
  onEdit,
  onDelete,
  revealed,
  onReveal,
}) {
  const nsfw = isNSFW(prompt.tags)
  const masked = blurNSFW && nsfw && !revealed
  const shown = prompt.tags.slice(0, 4)
  const rest = prompt.tags.length - shown.length

  return (
    <article className={`prompt-card${prompt.enabled ? '' : ' off'}`}>
      <div className="card-head">
        <h3 className="card-title" onClick={() => onOpen(prompt)} title={prompt.title}>
          {prompt.title || '（未命名）'}
        </h3>
        {isAdmin && (
          <span>
            <Tooltip title={prompt.favorite ? '取消收藏' : '加入收藏'}>
              <button
                type="button"
                className={`icon-btn${prompt.favorite ? ' fav' : ''}`}
                onClick={() => onFavorite(prompt)}
                aria-label="收藏"
              >
                {prompt.favorite ? <StarFilled /> : <StarOutlined />}
              </button>
            </Tooltip>
          </span>
        )}
        {!isAdmin && prompt.favorite && <StarFilled style={{ color: '#f59e0b', marginTop: 3 }} />}
      </div>

      <div className="card-tags">
        {shown.map((t) => (
          <span key={t} className={`mini-tag${isNSFW([t]) ? ' nsfw' : ''}`}>
            {t}
          </span>
        ))}
        {rest > 0 && <span className="mini-tag plain">+{rest}</span>}
      </div>

      <div className="preview-wrap">
        <div
          className={`card-preview${masked ? ' masked' : ''}`}
          onClick={() => (masked ? onReveal(prompt.id) : onOpen(prompt))}
        >
          {prompt.content}
        </div>
        {masked && (
          <div className="mask-overlay" onClick={() => onReveal(prompt.id)}>
            <EyeInvisibleOutlined style={{ marginRight: 6 }} />
            点击显示内容
          </div>
        )}
      </div>

      <div className="card-foot">
        <span className="card-meta">
          {prompt.source ? `#${prompt.source} · ` : ''}
          {formatTime(prompt.updated_at || prompt.created_at)}
        </span>
        <Tooltip title="复制提示词">
          <button type="button" className="icon-btn" onClick={() => onCopy(prompt)}>
            <CopyOutlined />
          </button>
        </Tooltip>
        {masked ? (
          <Tooltip title="显示内容">
            <button type="button" className="icon-btn" onClick={() => onReveal(prompt.id)}>
              <EyeOutlined />
            </button>
          </Tooltip>
        ) : null}
        {isAdmin && (
          <>
            <Tooltip title="编辑">
              <button type="button" className="icon-btn" onClick={() => onEdit(prompt)}>
                <EditOutlined />
              </button>
            </Tooltip>
            <Popconfirm
              title="确定删除这条提示词？"
              okText="删除"
              cancelText="取消"
              okButtonProps={{ danger: true }}
              onConfirm={() => onDelete(prompt)}
            >
              <Tooltip title="删除">
                <button type="button" className="icon-btn">
                  <DeleteOutlined />
                </button>
              </Tooltip>
            </Popconfirm>
          </>
        )}
      </div>
    </article>
  )
}
