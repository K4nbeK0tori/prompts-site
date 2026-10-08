import React from 'react'
import { Alert, Form, Input, Modal } from 'antd'
import { LockOutlined } from '@ant-design/icons'

export default function LoginModal({ open, loading, error, onCancel, onSubmit }) {
  const [form] = Form.useForm()

  React.useEffect(() => {
    if (open) form.resetFields()
  }, [open, form])

  const handleOk = async () => {
    const values = await form.validateFields()
    onSubmit(values.password)
  }

  return (
    <Modal
      open={open}
      title="管理员登录"
      okText="登录"
      cancelText="取消"
      confirmLoading={loading}
      onOk={handleOk}
      onCancel={onCancel}
      width={400}
      destroyOnClose
    >
      <p style={{ color: 'var(--ink-2)', fontSize: 13, marginTop: 0 }}>
        站点公开可读；登录后才可以新增、编辑和删除。
      </p>
      {error ? <Alert type="error" showIcon message={error} style={{ marginBottom: 12 }} /> : null}
      <Form form={form} layout="vertical" onFinish={handleOk}>
        <Form.Item name="password" rules={[{ required: true, message: '请输入管理员密码' }]}>
          <Input.Password
            prefix={<LockOutlined style={{ color: 'var(--accent)' }} />}
            placeholder="管理员密码"
            autoFocus
            onPressEnter={handleOk}
          />
        </Form.Item>
      </Form>
    </Modal>
  )
}
