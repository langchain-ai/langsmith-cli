import { createContext, useContext } from 'react';

import zIndices from './zIndices';

/**
 * Tracks the z-index of the nearest portal-rendered ancestor (pane, dialog,
 * popover, etc.). Defaults to zIndices.content (0) when there is no ancestor.
 *
 * Consumers should never read this context directly. Use useZIndex() instead.
 */
export const ZIndexContext = createContext<number>(zIndices.content);

/**
 * Returns the z-index a portal-rendered overlay should use so that it always
 * stacks above its nearest ancestor overlay, regardless of nesting depth.
 *
 * Portal-rendered elements (popovers, dropdowns, dialogs) escape the DOM tree
 * and render at the document body, so their z-index competes globally. Without
 * coordination a dropdown inside a modal can render behind it if the dropdown's
 * default z-index is lower than the modal's.
 *
 * useZIndex solves this with Math.max(defaultZIndex, contextZIndex + 1):
 * - At the root (no ancestor): uses defaultZIndex, the semantic floor that
 *   guarantees the overlay beats all page layout (e.g. popover always >= 1300).
 * - Inside an ancestor overlay: ignores defaultZIndex and uses contextZIndex + 1
 *   instead, ensuring it always renders ab ove the parent regardless of the
 *   parent's absolute value.
 *
 * After calling useZIndex, the component must:
 * 1. Apply the returned value as style={{ zIndex }} on the portaled element.
 * 2. Wrap its children in <ZIndexProvider value={zIndex}> so that any further
 *    nested overlays can repeat the same calculation relative to this layer.
 *
 * Note: Tooltips do NOT use useZIndex — they have no semantic floor and always
 * render at contextZIndex + 1 directly, since they only need to appear above
 * whatever triggered them.
 *
 * @example
 * ```tsx
 * function MyPopover({ children }) {
 *   const zIndex = useZIndex(zIndices.popover);
 *   return (
 *     <Portal>
 *       <Content style={{ zIndex }}>
 *         <ZIndexProvider value={zIndex}>
 *           {children}
 *         </ZIndexProvider>
 *       </Content>
 *     </Portal>
 *   );
 * }
 * ```
 *
 * @param defaultZIndex - The semantic floor for this overlay type (e.g. zIndices.popover).
 * @returns The z-index to apply — either the floor or contextZIndex + 1, whichever is higher.
 */
export function useZIndex(defaultZIndex: number): number {
  const contextZIndex = useContext(ZIndexContext);
  return Math.max(defaultZIndex, contextZIndex + 1);
}
