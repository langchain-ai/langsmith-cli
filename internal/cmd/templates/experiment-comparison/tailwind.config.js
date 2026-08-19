/** @type {import('tailwindcss').Config} */
export default {
  // Every semantic token — bg-surface-level-*, text-primary, border-default,
  // gap-space-*, rounded-*, shadow-*, the animation keyframes — comes from the
  // LangSmith design-system preset. Refresh it with
  // `npx shadcn add --overwrite @langsmith/theme`.
  //
  // Don't re-declare the preset's scales in `theme.extend` here: a local key of
  // the same name wins, so a stale copy of e.g. the spacing scale silently
  // restyles every design-system component.
  presets: [require('./tailwind.langsmith.cjs')],
  content: ['./src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
};
