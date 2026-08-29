/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./cmd/goisekai/frontend/**/*.{html,js}'],
  theme: {
    extend: {
      colors: {
        bg: '#0f0f14',
        surface: { DEFAULT: '#1a1a24', alt: '#24243a', elevated: '#2a2a45' },
        accent: { DEFAULT: '#7c5cfc', hover: '#9478ff', subtle: 'rgba(124, 92, 252, 0.12)' },
        muted: '#71717a',
        success: '#4ade80',
        error: '#f87171',
        border: '#2a2a3e',
      },
      fontFamily: { sans: ['Inter', 'system-ui', 'sans-serif'] },
      borderRadius: { card: '0.5rem' },
      boxShadow: {
        glow: '0 0 20px rgba(124, 92, 252, 0.3)',
        'card-hover': '0 4px 24px rgba(0, 0, 0, 0.4)',
      },
    },
  },
  plugins: [],
}
