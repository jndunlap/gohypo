/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./ui/templates/**/*.{html,js}",
    "./ui/static/**/*.{js,css}",
    "./cmd/ui/*.go"
  ],
  theme: {
    extend: {
      colors: {
        // Tactical Color System - Status Indicators
        'void': '#020202',           // Background (The Void)
        'obsidian': '#0d0d0d',       // Containers (Obsidian)
        'cold-steel': '#222',        // Container borders
        'hyper-lime': '#00ff41',     // Truth Signal
        'crimson-ghost': '#ff003c',  // Error Signal
        'electric-cobalt': '#2e5bff', // System Infrastructure
        'ghost-white': '#f5f5f5',
        'faint-gray': '#111111',
      },
      fontFamily: {
        'inter': ['Inter', 'sans-serif'],
        'mono': ['JetBrains Mono', 'monospace'],
      },
      spacing: {
        '18': '4.5rem',
        '88': '22rem',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'glow': 'glow 2s ease-in-out infinite alternate',
      },
      keyframes: {
        glow: {
          '0%': { boxShadow: '0 0 5px rgba(0, 255, 65, 0.2)' },
          '100%': { boxShadow: '0 0 20px rgba(0, 255, 65, 0.4)' },
        },
      },
      backdropBlur: {
        xs: '2px',
      },
    },
  },
  plugins: [],
}