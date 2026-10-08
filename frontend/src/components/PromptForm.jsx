import React from 'react'
import { Form, Input, Modal, Select, Switch } from 'antd'

const SOURCE_OPTIONS = [
  { label: 'api', value: 'api' },
  { label: 'user', value: 'user' },
  { label: 'manual', value: 'manual' },
]

export default function PromptForm({ open, prompt, saving, onCancel, onSubmit }) {
  const [form] = Form.useForm()
  const isEdit = Boolean(prompt && prompt.id)

  React.useEffect(() => {
    if (!open) return
    if (prompt && prompt.id) {
      form.setFieldsValue({
        title: prompt.title,
        content: prompt.content,
        negative: prompt.negative,
        tags: prompt.tags,
        source: prompt.source || undefined,
        preview_url: prompt.preview_url,
        note: prompt.note,
        enabled: prompt.enabled,
        favorite: prompt.favorite,
      })
    } else {
      form.resetFields()
      form.setFieldsValue({ enabled: true, favorite: false, tags: [], source: 'manual' })
    }
  }, [open, prompt, form])

  const handleOk = async () => {
    const values = await form.validateFields()
    onSubmit({
      title: (values.title || '').trim(),
      content: (values.content || '').trim(),
      negative: values.negative || '',
      tags: values.tags || [],
      source: values.source || '',
      preview_url: values.preview_url || '',
      note: values.note || '',
      enabled: values.enabled !== false,
      favorite: Boolean(values.favorite),
    })
  }

  return (
    <Modal
      open={open}
      title={isEdit ? '编辑提示词' : '新增提示词'}
      okText={isEdit ? '保存' : '创建'}
      cancelText="取消"
      confirmLoading={saving}
      onOk={handleOk}
      onCancel={onCancel}
      width={720}
      destroyOnClose
      maskClosable={false}
    >
      <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 8 }}>
        <Form.Item
          name="title"
          label="名称"
          rules={[{ required: true, message: '给这条提示词起个名字吧' }]}
        >
          <Input placeholder="例如：电梯露出内裤" maxLength={80} showCount />
        </Form.Item>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'nowrap' }}>
          <Form.Item name="tags" label="标签" style={{ flex: 1 }} tooltip="回车添加，可用中文顿号分隔">
            <Select
              mode="tags"
              placeholder="img2img、NSFW、banana"
              tokenSeparators={['、', ',', '，', ' ']}
              options={[]}
            />
          </Form.Item>
          <Form.Item name="source" label="来源" style={{ width: 150 }}>
            <Select allowClear placeholder="选择来源" options={SOURCE_OPTIONS} />
          </Form.Item>
        </div>

        <Form.Item
          name="content"
          label="正向提示词"
          rules={[{ required: true, message: '提示词内容不能为空' }]}
        >
          <Input.TextArea rows={10} placeholder="粘贴完整提示词…" style={{ fontFamily: 'Consolas, monospace' }} />
        </Form.Item>

        <Form.Item name="negative" label="负面提示词">
          <Input.TextArea rows={3} placeholder="可留空" style={{ fontFamily: 'Consolas, monospace' }} />
        </Form.Item>

        <Form.Item name="preview_url" label="预览图外链" tooltip="只存链接，图片不落服务器磁盘">
          <Input placeholder="https://…" />
        </Form.Item>

        <Form.Item name="note" label="备注">
          <Input.TextArea rows={2} placeholder="可留空" />
        </Form.Item>

        <div style={{ display: 'flex', gap: 32 }}>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="favorite" label="收藏" valuePropName="checked">
            <Switch />
          </Form.Item>
        </div>
      </Form>
    </Modal>
  )
}
