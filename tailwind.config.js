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
      // Custom typography for markdown rendering
      typography: ({ theme }) => ({
        invert: {
          css: {
            '--tw-prose-body': theme('colors.gray[300]'),
            '--tw-prose-headings': theme('colors.neon.cyan'),
            '--tw-prose-links': theme('colors.neon.cyan'),
            '--tw-prose-bold': theme('colors.gray[100]'),
            '--tw-prose-counters': theme('colors.gray[400]'),
            '--tw-prose-bullets': theme('colors.neon.magenta'),
            '--tw-prose-hr': theme('colors.hive.metal'),
            '--tw-prose-quotes': theme('colors.gray[400]'),
            '--tw-prose-quote-borders': theme('colors.neon.magenta'),
            '--tw-prose-captions': theme('colors.gray[400]'),
            '--tw-prose-code': theme('colors.neon.yellow'),
            '--tw-prose-pre-code': theme('colors.gray[300]'),
            '--tw-prose-pre-bg': theme('colors.hive.surface'),
            '--tw-prose-th-borders': theme('colors.hive.metal'),
            '--tw-prose-td-borders': theme('colors.hive.metal'),
            // Prevent long URLs and words from breaking layout
            'word-wrap': 'break-word',
            'overflow-wrap': 'break-word',
            'word-break': 'break-word',
            // Links should break to prevent overflow
            'a': {
              'word-break': 'break-all',
              'overflow-wrap': 'anywhere',
            },
            // Code blocks should scroll horizontally instead of breaking layout
            'pre': {
              'overflow-x': 'auto',
              'word-wrap': 'normal',
              'word-break': 'normal',
            },
            // Make headings smaller and less prominent
            'h1': {
              fontSize: '1.25rem',
              fontWeight: '600',
            },
            'h2': {
              fontSize: '1.125rem',
              fontWeight: '600',
            },
            'h3': {
              fontSize: '1rem',
              fontWeight: '600',
            },
            'h4': {
              fontSize: '0.95rem',
              fontWeight: '600',
            },
            'h5': {
              fontSize: '0.9rem',
              fontWeight: '600',
            },
            'h6': {
              fontSize: '0.85rem',
              fontWeight: '600',
            },
            // Reduce spacing between list items
            'ul > li': {
              marginTop: '0.25rem',
              marginBottom: '0.25rem',
            },
            'ol > li': {
              marginTop: '0.25rem',
              marginBottom: '0.25rem',
            },
          },
        },
      }),
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
}
