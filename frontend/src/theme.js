// 樱花主题：配色直接对齐 KotoriVPN 面板
export const FONT_STACK =
  "'PingFang SC','HarmonyOS Sans SC','Microsoft YaHei','Hiragino Sans GB',system-ui,-apple-system,'Segoe UI',sans-serif"

export const MONO_STACK =
  "'JetBrains Mono','Cascadia Code',Consolas,'SF Mono',Menlo,monospace"

const shared = {
  borderRadius: 14,
  fontFamily: FONT_STACK,
  fontSize: 14,
  controlHeight: 36,
  wireframe: false,
}

export const lightTheme = {
  token: {
    ...shared,
    colorPrimary: '#ec4899',
    colorInfo: '#ec4899',
    colorLink: '#ec4899',
    colorSuccess: '#34d399',
    colorWarning: '#fbbf24',
    colorError: '#f43f5e',
    colorBgBase: '#fff5f9',
    colorBgLayout: '#fdf2f8',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorBorder: '#fbcfe8',
    colorBorderSecondary: '#fce7f3',
    colorText: '#3f2a38',
    colorTextSecondary: '#8b7280',
    colorTextTertiary: '#a892a0',
    boxShadowSecondary: '0 6px 24px rgba(236,72,153,0.10)',
  },
  components: {
    Card: { borderRadiusLG: 18 },
    Modal: { borderRadiusLG: 20 },
    Button: { borderRadius: 12, primaryShadow: '0 4px 14px rgba(236,72,153,0.28)' },
    Input: { borderRadius: 12 },
    Select: { borderRadius: 12 },
    Tag: { borderRadiusSM: 8 },
    Drawer: { paddingLG: 22 },
  },
}

export const darkTheme = {
  token: {
    ...shared,
    colorPrimary: '#f472b6',
    colorInfo: '#f472b6',
    colorLink: '#f472b6',
    colorSuccess: '#34d399',
    colorWarning: '#fbbf24',
    colorError: '#fb7185',
    colorBgBase: '#191420',
    colorBgLayout: '#191420',
    colorBgContainer: '#221a2e',
    colorBgElevated: '#2a2138',
    colorBorder: '#3b2d4d',
    colorBorderSecondary: '#2f2540',
    colorText: '#f3e8ff',
    colorTextSecondary: '#b9a7c9',
    colorTextTertiary: '#8f7fa3',
    boxShadowSecondary: '0 6px 24px rgba(0,0,0,0.35)',
  },
  components: {
    Card: { borderRadiusLG: 18 },
    Modal: { borderRadiusLG: 20 },
    Button: { borderRadius: 12, primaryShadow: '0 4px 14px rgba(244,114,182,0.25)' },
    Input: { borderRadius: 12 },
    Select: { borderRadius: 12 },
    Tag: { borderRadiusSM: 8 },
    Drawer: { paddingLG: 22 },
  },
}
