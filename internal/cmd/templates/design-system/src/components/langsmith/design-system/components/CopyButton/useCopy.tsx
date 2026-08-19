import { useEffect, useRef, useState } from 'react';

import { copyToClipboard } from './copyToClipboard';

// Alternative to useCopy, where the copy text is provided as an argument

const copyWithErrorLogging = (copy: string | undefined) => {
  void copyToClipboard(copy).catch((error: unknown) => {
    console.error(error);
  });
};

export const useCopyAction = () => {
  const [copied, setCopied] = useState(false);

  const timerRef = useRef<number>(undefined);
  const onCopy = (copy: string) => {
    window.clearTimeout(timerRef.current);
    copyWithErrorLogging(copy);
    setCopied(true);
    timerRef.current = window.setTimeout(() => setCopied(false), 2000);
  };

  useEffect(() => {
    return () => window.clearTimeout(timerRef.current);
  }, []);

  return { copied, onCopy };
};

export const useCopy = (props?: { copy?: string }) => {
  const [copied, setCopied] = useState(false);

  const timerRef = useRef<number>(undefined);
  const onCopy = () => {
    window.clearTimeout(timerRef.current);
    copyWithErrorLogging(props?.copy);
    setCopied(true);
    timerRef.current = window.setTimeout(() => setCopied(false), 2000);
  };

  useEffect(() => {
    return () => window.clearTimeout(timerRef.current);
  }, []);

  return { copied, onCopy };
};
