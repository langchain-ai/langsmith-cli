import { useState } from 'react';
import { Banner } from '@langchain/macaw-components/Banner';
import { IconButton } from '@langchain/macaw-components/IconButton';
import { CaretDownIcon, CaretRightIcon } from '@langchain/macaw-components/icons';

export function ErrorBanner({ error }: { error: string }) {
  const [expanded, setExpanded] = useState(false);
  const canExpand = error.includes('\n') || error.length > 100;
  return (
    <div role="alert" className="mb-space-3">
      <Banner
        intent="error"
        title="Error"
        action={
          canExpand ? (
            <IconButton
              color="secondary"
              variant="plain"
              icon={expanded ? CaretDownIcon : CaretRightIcon}
              label={expanded ? 'Collapse error' : 'Expand error'}
              onClick={() => setExpanded((value) => !value)}
            />
          ) : undefined
        }
      >
        <span className={expanded ? 'whitespace-pre-wrap break-words' : 'line-clamp-1'}>
          {error}
        </span>
      </Banner>
    </div>
  );
}
