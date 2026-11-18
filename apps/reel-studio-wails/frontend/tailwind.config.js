/**
 * tailwind.config.js —— muse-tv 桌面控制台的设计令牌（唯一真源）。
 *
 * 方向（用户 2026-11-17 定）：高级深色、大气；灰色冷光；像一束光从上方打下来。
 *
 *   底色是近黑的冷灰（低饱和，不发蓝不发紫），层级靠「离光近不近」拉开：
 *   越靠上、越被光照到的面越亮一点。光是银白色的冷光，不是彩色霓虹。
 *   唯一被光正面打亮的物体是主按钮 —— 银白实体，深字；其余控件都在暗处。
 *   强调色只用一点淡钢蓝，做焦点、选中、进行中，不铺大面。
 */
module.exports = {
  content: ['./src/**/*.{html,js,vue}'],
  theme: {
    extend: {
      colors: {
        // 底色四层：暗室 → 面 → 浮起 → 受光面
        paper: '#0b0c0e',
        surface: '#121417',
        raised: '#181b1f',
        lit: '#20242a',

        // 文字四档（在 paper 上对比度：主 15:1 / 次 8:1 / 弱 5:1 / 极弱 3:1，极弱只用于占位与装饰）
        ink: {
          DEFAULT: '#e8eaed',
          mid: '#a9afb8',
          lo: '#848b95',
          dim: '#5f6670',
        },

        line: {
          DEFAULT: '#23272d',
          strong: '#323840',
        },

        // 冷光强调：淡钢蓝，克制使用
        accent: {
          DEFAULT: '#9dbbd8',
          deep: '#6f93b8',
          soft: '#16202a',
        },

        // 受光的银白：主按钮材质
        silver: {
          hi: '#f3f5f7',
          DEFAULT: '#d9dde2',
          lo: '#aeb4bc',
        },

        ok: '#6fb394',
        warn: '#d1a64a',
        err: '#e0786b',

        // 画面台：最深
        stage: '#050607',
      },

      borderRadius: {
        sm: '4px',
        md: '6px',
        lg: '10px',
      },

      fontSize: {
        '2xs': ['11px', { lineHeight: '1.5' }],
        xs: ['12px', { lineHeight: '1.6' }],
        sm: ['13px', { lineHeight: '1.6' }],
        base: ['14px', { lineHeight: '1.6' }],
        lg: ['16px', { lineHeight: '1.45' }],
        xl: ['19px', { lineHeight: '1.35' }],
        '2xl': ['23px', { lineHeight: '1.25' }],
      },

      spacing: {
        4.5: '18px',
        13: '52px',
        18: '72px',
      },

      boxShadow: {
        // 面板：顶边一道受光细线 + 落在暗处的投影
        card: 'inset 0 1px 0 rgba(255,255,255,.05), 0 1px 0 rgba(0,0,0,.4), 0 8px 24px -12px rgba(0,0,0,.6)',
        // 主按钮：被光照亮的银白实体
        lit: 'inset 0 1px 0 rgba(255,255,255,.9), inset 0 -1px 0 rgba(0,0,0,.18), 0 0 0 1px rgba(255,255,255,.06), 0 6px 22px -6px rgba(220,228,238,.28)',
        'lit-hover': 'inset 0 1px 0 rgba(255,255,255,1), inset 0 -1px 0 rgba(0,0,0,.18), 0 0 0 1px rgba(255,255,255,.1), 0 8px 30px -6px rgba(220,228,238,.42)',
        pressed: 'inset 0 2px 6px rgba(0,0,0,.35)',
        pop: 'inset 0 1px 0 rgba(255,255,255,.06), 0 24px 64px rgba(0,0,0,.65), 0 2px 8px rgba(0,0,0,.4)',
        stage: '0 0 0 1px rgba(255,255,255,.04), 0 18px 48px -12px rgba(0,0,0,.8)',
        // 选中/当前：一圈冷光
        halo: '0 0 0 1px rgba(157,187,216,.28), 0 0 18px -4px rgba(157,187,216,.25)',
      },

      backgroundImage: {
        // 面板受光：顶部略亮，向下沉入暗处
        'surface-lit': 'linear-gradient(180deg, rgba(255,255,255,.035) 0%, rgba(255,255,255,0) 42%)',
        // 主按钮的银白金属面
        'silver-face': 'linear-gradient(180deg, #f3f5f7 0%, #d9dde2 55%, #c3c9d0 100%)',
        'silver-face-hover': 'linear-gradient(180deg, #ffffff 0%, #e4e7eb 55%, #ccd2d8 100%)',
      },

      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', '"PingFang SC"', '"Helvetica Neue"', 'sans-serif'],
        serif: ['Georgia', '"Songti SC"', 'serif'],
        mono: ['"SF Mono"', 'ui-monospace', 'Menlo', 'monospace'],
      },
    },
  },
  plugins: [],
}
