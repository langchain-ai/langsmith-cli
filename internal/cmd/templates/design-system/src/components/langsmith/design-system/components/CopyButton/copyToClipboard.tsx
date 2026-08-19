export async function copyToClipboard(textToCopy: string | undefined) {
  if (textToCopy == null) return;

  // This helper intentionally stays hook-free so non-component utilities can copy text too.
  // Navigator clipboard api needs a secure context (https)
  // eslint-disable-next-line custom/prefer-mantine-hooks
  if (navigator.clipboard && window.isSecureContext) {
    // eslint-disable-next-line custom/prefer-mantine-hooks
    await navigator.clipboard.writeText(textToCopy);
  } else {
    const focusedElement =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;

    const textArea = document.createElement('textarea');
    textArea.value = textToCopy;

    textArea.style.position = 'absolute';
    textArea.style.opacity = '0';
    textArea.style.height = '0';
    textArea.style.width = '0';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
      document.execCommand('copy');
    } catch (err) {
      console.error(err);
    } finally {
      document.body.removeChild(textArea);
      focusedElement?.focus({ preventScroll: true });
    }
  }
}
