/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./web/templates/**/*.html",
    "./web/templates/**/*.tmpl",
  ],
  theme: {
    extend: {
      // Dark cyberpunk theme colors
      colors: {
        hive: {
          // The deepest background color, sampled from the darkest server shadows
          bg: '#111318',
          // A slightly lighter shade for cards, sidebars, or inputs
          surface: '#1f222e',
          // A muted gray-blue for secondary text or borders, matching the server metal
          metal: '#374151',
          // The color of the brain matter, useful for muted highlights
          brain: '#c48f94',
        },
        // High-contrast neon accents for the "glitch" effect
        neon: {
          cyan: '#00ffff',
          magenta: '#ff00ff',
          // The yellow from the sparks and sticky note
          yellow: '#fde047',
          // The red from the blinking server lights
          alert: '#ef4444'
        }
      },
      // Custom font stacks
      fontFamily: {
        // For headings, matching the blocky "HIVEMIND" look
        heading: ['"Montserrat"', '"Orbitron"', 'sans-serif'],
        // For body text, a clean technical monospace fits the "wiki" vibe
        mono: ['"JetBrains Mono"', '"Fira Code"', 'monospace'],
        // Standard readable body font (keeping Reddit Sans)
        sans: ['"Reddit Sans"', 'Inter', 'sans-serif'],
      },
      // Custom drop shadows for neon glow effects
      boxShadow: {
        'neon-cyan': '0 0 10px rgba(0, 255, 255, 0.5), 0 0 20px rgba(0, 255, 255, 0.3)',
        'neon-magenta': '0 0 10px rgba(255, 0, 255, 0.5), 0 0 20px rgba(255, 0, 255, 0.3)',
      },
    },
  },
  plugins: [],
}
