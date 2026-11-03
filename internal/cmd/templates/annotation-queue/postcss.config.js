import postcss from 'postcss';
import tailwindcss from 'tailwindcss';
import autoprefixer from 'autoprefixer';

export default {
  plugins: [
    {
      postcssPlugin: 'starter-tailwind',
      async Once(root, { result }) {
        // Macaw ships compiled CSS with native layers. Only compile our
        // stylesheet, which opts in with @tailwind directives.
        const usesTailwind = root.nodes.some(
          (node) => node.type === 'atrule' && node.name === 'tailwind',
        );
        if (!usesTailwind) return;
        const compiled = await postcss([tailwindcss()]).process(root, result.opts);
        result.messages.push(...compiled.messages);
      },
    },
    autoprefixer(),
  ],
};
