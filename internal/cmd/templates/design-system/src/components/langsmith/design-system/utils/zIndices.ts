/**
 * Centralized z-index values for the application.
 *
 * This file defines all z-index values used throughout the app to ensure
 * proper stacking order and avoid conflicts.
 *
 * Z-index layers (from lowest to highest):
 * - Content (0): Natural CSS baseline — regular content, sticky headers
 * - Navigation (1): Resize handles, floating bars
 * - Layout (2): Sidebar, overlays
 * - Grid interaction (4-5): Drag/resize overlays, sticky page headers
 * - Pane (500): Side panes
 * - Popover (1000+): Popovers, dropdowns — large intentional jump to guarantee
 *   portal-rendered overlays always beat any page layout complexity
 * - Modal (2000+): Modal dialogs
 * - Toast (9999): System toasts
 *
 * Note: Tooltips do NOT have a fixed z-index here. They always render at
 * contextZIndex + 1 (relative to their parent caller) via ZIndexContext,
 * so they naturally stack above whichever layer triggered them.
 */

const zIndices = {
  // Content layer
  content: 0,
  stickyHeader: 1,
  tableStickyCell: 1, // Sticky table columns — above non-sticky cells (auto)
  tableStickyHeader: 3, // Sticky table header — above rows and sticky columns
  paneHeader: 4, // Above a scrolled table's sticky header
  dragOverlay: 4, // Grid drag/resize overlays — above sticky table headers (3)
  dashboardHeader: 5, // Sticky page header — above drag overlays, below tooltips
  floatingBar: 2,
  resizeHandle: 2, // Higher than sticky header to ensure it's always interactive

  // Layout layer (this is arbitrarily 5 to give a few layers above the content layer)
  sidebar: 5,

  tooltip: 6,

  selectionActionBar: 12, // Selection bars — above page headers and their tooltips

  // Pane layer
  pane: 500,

  // Popover layer — large jump ensures portal overlays always beat page layout
  popover: 1300,

  // Dev tool panels (e.g. Eppo control panel) — above popovers/Polly, below dialogs
  devToolPanel: 2500,

  dialogOverlay: 2999,
  dialogContent: 3000,

  // System toasts — above dialogs, but bounded so nested items(e.g., button labels) can overlay
  toast: 9999,
} as const;

export default zIndices;
