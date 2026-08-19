import React from 'react';

import type { ToastConfig } from '.';

export const ToastContext = React.createContext<{
  createToast: (titleOrConfig: ToastConfig | string) => void;
}>({
  createToast: () => {
    console.error('ToastContext not initialized');
  },
});
