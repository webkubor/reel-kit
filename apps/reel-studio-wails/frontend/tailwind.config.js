/**
 * tailwind.config.js —— Reel Studio 桌面控制台的设计令牌。
 *
 * 这套语言是给「桌面工具」定的，不是给网页定的：
 *
 *   界面是浅色纸面（安静、像一张工作台），
 *   画面是深色（成片本来就是深底，见 templates/product-presenter.html），
 *   所以画面是整个界面里唯一「重」的物体 —— 视线自然落在它上面。
 *
 * 薄荷绿与浅粉是浅色材质；深绿仅用于文字、选中标记和焦点。
 * 主按钮保持高明度，以乳白过渡连接两端，避免深绿与莓红混出脏灰。
 */
module.exports = {
  content: ['./src/**/*.{html,js}'],
  theme: {
    extend: {
      colors: {
        // 界面底色：纸 → 面 → 浮起，三层就够，再多就没人分得清
        paper: '#f2f8f5',
        surface: '#ffffff',
        raised: '#f8f3f5',
        blush: '#f9e9ef',

        // 文字四档：主 / 次 / 弱 / 极弱
        ink: {
          DEFAULT: '#22252a',
          mid: '#5b6169',
          lo: '#616873',
          dim: '#6c7178',
        },

        // 描边两档
        line: {
          DEFAULT: '#e4e3dd',
          strong: '#d2d1ca',
        },

        // 深绿用于可读文字；主操作材质用高明度薄荷、乳白与浅粉。
        accent: {
          DEFAULT: '#176958',
          deep: '#115348',
          soft: '#e0f3eb',
        },
        glass: {
          mint: '#c8f2e1',
          milk: '#f2faf6',
          pink: '#fbe0ea',
          edge: '#b4d2c8',
          'mint-hover': '#b7ead4',
          'pink-hover': '#f9d4e2',
        },

        ok: '#3f7d5c',
        warn: '#a8761f',
        err: '#b53f34',

        // 画面台底色，跟 templates/product-presenter.html 的 body 一致
        stage: '#0d0f11',
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
        card: '0 1px 2px rgba(34,37,42,.05)',
        glass: 'inset 0 1px 0 rgba(255,255,255,.95), inset 0 -1px 0 rgba(23,105,88,.04), 0 2px 4px rgba(23,105,88,.08)',
        'glass-pressed': 'inset 0 1px 3px rgba(23,105,88,.12)',
        pop: '0 10px 30px rgba(34,37,42,.12)',
        stage: '0 3px 14px rgba(13,15,17,.20)',
      },

      fontFamily: {
        sans: [
          '-apple-system',
          'BlinkMacSystemFont',
          '"PingFang SC"',
          '"Helvetica Neue"',
          'sans-serif',
        ],
        serif: ['Georgia', '"Songti SC"', 'serif'],
        mono: ['"SF Mono"', 'ui-monospace', 'Menlo', 'monospace'],
      },
    },
  },
  plugins: [],
}
