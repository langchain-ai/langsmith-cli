import React from 'react';

import { ToastContext } from './ToastContext';

export default function useToast() {
  const { createToast } = React.useContext(ToastContext);
  return { createToast };
}
