import React from 'react'
import { Button, Drawer, Popconfirm, Space, Tag, message } from 'antd'
import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  StarFilled,
  StarOutlined,
} from '@ant-design/icons'
import { formatTime, isNSFW } from './PromptCard'

function CopyButton({ text, label = '复制' }) {
  return (
    <Button
      size="small"
      type="text"
      icon={<CopyOutlined />}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text || '')
          message.success('已复制到剪贴板')
        } catch {
          message.error('复制失败，请手动选择文本')
        }
      }}
    >
      {label}
    </Button>
  )
}

export default function PromptDetail({ prompt, open, onClose, isAdmin, onEdit, onDelete, onFavorite }) {
  if (!prompt) return null

  return (
    <Drawer
      open={open}
      onClose={onClose}
      width={Math.min(760, typeof window !== 'undefined' ? window.innerWidth - 32 : 760)}
      title={
        <Space size={8} wrap>
          <span style={{ fontWeight: 650 }}>{prompt.title || '（未命名）'}</span>
          {prompt.favorite && <StarFilled style={{ color: '#f59e0b' }} />}
        </Space>
      }
      extra={
        isAdmin ? (
          <Space size={4}>
            <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(prompt)}>
              编辑
            </Button>
            <Popconfirm
              title="确定删除这条提示词？"
              okText="删除"
              cancelText="取消"
              okButtonProps={{ danger: true }}
              onConfirm={() => onDelete(prompt)}
            >
              <Button size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ) : null
      }
    >
      <div className="detail-meta">
        {prompt.tags.map((t) => (
          <Tag key={t} color={isNSFW([t]) ? 'red' : 'magenta'} bordered={false}>
            {t}
          </Tag>
        ))}
        {prompt.source && <Tag bordered={false}>{prompt.source}</Tag>}
        <Tag color={prompt.enabled ? 'green' : 'default'} bordered={false}>
          {prompt.enabled ? '已启用' : '已停用'}
        </Tag>
      </div>

      <div className="detail-label">
        <span>正向提示词</span>
        <span style={{ display: 'flex', gap: 4 }}>
          <CopyButton text={prompt.content} label="复制" />
          {isAdmin && (
            <Button
              size="small"
              type="text"
              icon={prompt.favorite ? <StarFilled /> : <StarOutlined />}
              onClick={() => onFavorite(prompt)}
            >
              {prompt.favorite ? '取消收藏' : '收藏'}
            </Button>
          )}
        </span>
      </div>
      <div className="detail-block">{prompt.content}</div>

      {prompt.negative ? (
        <>
          <div className="detail-label">
            <span>负面提示词</span>
            <CopyButton text={prompt.negative} />
          </div>
          <div className="detail-block">{prompt.negative}</div>
        </>
      ) : null}

      {prompt.note ? (
        <>
          <div className="detail-label">
            <span>备注</span>
          </div>
          <div className="detail-block">{prompt.note}</div>
        </>
      ) : null}

      {prompt.preview_url ? (
        <>
          <div className="detail-label">
            <span>预览图外链</span>
          </div>
          <a href={prompt.preview_url} target="_blank" rel="noreferrer noopener">
            {prompt.preview_url}
          </a>
        </>
      ) : null}

      <div className="detail-label">
        <span>时间</span>
      </div>
      <div style={{ fontSize: 12.5, color: 'var(--ink-2)' }}>
        创建：{formatTime(prompt.created_at)} · 更新：{formatTime(prompt.updated_at)}
      </div>
    </Drawer>
  )
}
