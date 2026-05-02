// Phase 15 Plan 15-05 — BreakerConfirmModal (PG-LIVE-02 frontend).
//
// Shared typed-phrase modal used by BreakerSubsection for both recover and
// test-fire actions. Enforces an EXACT case-sensitive match against the
// magic phrase (RECOVER or TEST-FIRE) before enabling the Confirm button.
// Server re-validates server-side per T-15-13 (defense in depth).
//
// CONTEXT D-12: magic strings RECOVER / TEST-FIRE are LITERAL — they remain
// the same regardless of locale. Translatable text is the surrounding prompt
// only.
//
// Test-fire variant exposes a Dry-Run checkbox; when unchecked, the
// REAL-TRIP warning banner renders prominently above the typed-phrase
// input (CONTEXT "Pitfall 7" / Plan 15-04 mitigation).
import { useEffect, useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import type { TranslationKey } from '../../i18n/index.ts';

export type BreakerConfirmAction = 'recover' | 'test-fire';

// Phase 16 Plan 04 (PG-OPS-09 D-18): Phrase union extended to cover the new
// Strategy 4 Configuration card toggles (live-capital + breaker enable). Magic
// strings are LITERAL — never translated, never lowercased, never trimmed.
// Interaction logic (case-sensitive match, Esc closes, Enter does not submit,
// real-trip warning banner) is unchanged from the Phase 15 contract.
export type Phrase = 'RECOVER' | 'TEST-FIRE' | 'ENABLE-LIVE-CAPITAL' | 'ENABLE-BREAKER';

interface Props {
  open: boolean;
  action: BreakerConfirmAction;
  magicPhrase: Phrase;
  promptKey: TranslationKey;
  busy: boolean;
  errorMessage?: string | null;
  onClose: () => void;
  onConfirm: (opts: { dryRun: boolean }) => void;
  // Phase 16 Plan 04 — optional overrides for the Strategy 4 Configuration
  // card. When omitted the modal falls back to the Phase 15 recover/test-fire
  // copy as before (no behavior change for existing callers).
  titleKey?: TranslationKey;
  submitKey?: TranslationKey;
  // forceWarning: when true, render the real-trip red banner regardless of the
  // dry-run checkbox (used for ENABLE-LIVE-CAPITAL where dry-run is not
  // exposed and the banner is permanent).
  forceWarning?: boolean;
  // hideDryRun: when true, suppress the dry-run checkbox (Strategy 4
  // Configuration toggles do not have a dry-run concept).
  hideDryRun?: boolean;
  // destructiveSubmit: when true, render the submit button red regardless of
  // dry-run state — used for live-capital ON / breaker disable.
  destructiveSubmit?: boolean;
}

export const BreakerConfirmModal: FC<Props> = ({
  open,
  action,
  magicPhrase,
  promptKey,
  busy,
  errorMessage,
  onClose,
  onConfirm,
  titleKey,
  submitKey,
  forceWarning,
  hideDryRun,
  destructiveSubmit,
}) => {
  const { t } = useLocale();
  const [typed, setTyped] = useState('');
  const [dryRun, setDryRun] = useState(false);

  // Reset on open. Default dry-run to FALSE so the operator must opt INTO
  // dry-run — the safer default is the explicit one (D-12 Pitfall 7).
  useEffect(() => {
    if (open) {
      setTyped('');
      setDryRun(false);
    }
  }, [open, action]);

  // Esc key closes (matches PriceGap.tsx modal pattern).
  useEffect(() => {
    if (!open) return;
    const h = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !busy) onClose();
    };
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, [open, busy, onClose]);

  if (!open) return null;

  // EXACT case-sensitive match. No trim, no toUpperCase. T-15-13 mitigation:
  // pasting a lowercased copy from chat history fails closed.
  // Phase 16 Plan 04: when magicPhrase is empty the modal is in "no-phrase"
  // mode (used for safer-direction toggles like Disable live-capital). The
  // confirm button is enabled by default and the input is hidden by the
  // caller via empty magicPhrase. We treat empty magicPhrase as
  // unconditionally valid.
  const noPhrase = magicPhrase === ('' as unknown as Phrase);
  const valid = noPhrase || typed === magicPhrase;
  const isTestFire = action === 'test-fire';
  const showRealTripWarning = (isTestFire && !dryRun) || forceWarning === true;
  const showDryRun = isTestFire && !hideDryRun;

  const resolvedTitleKey: TranslationKey =
    titleKey ?? (isTestFire
      ? 'pricegap.breaker.confirmTitleTestFire'
      : 'pricegap.breaker.confirmTitleRecover');
  const resolvedSubmitKey: TranslationKey = submitKey ?? 'pricegap.breaker.confirmSubmit';

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={() => {
        if (!busy) onClose();
      }}
    >
      <div
        role="dialog"
        aria-labelledby="pg-breaker-confirm-title"
        className="bg-gray-800 border border-gray-700 rounded-lg shadow-xl w-full max-w-md p-5"
        onClick={(e) => e.stopPropagation()}
        data-test="breaker-confirm-modal"
      >
        <h3
          id="pg-breaker-confirm-title"
          className="text-lg font-semibold text-white mb-3"
          data-test="breaker-confirm-title"
        >
          {t(resolvedTitleKey)}
        </h3>

        {/* Test-fire only: Dry-run checkbox. Place ABOVE the warning so a
            late-stage flip of the checkbox visibly clears the warning. */}
        {showDryRun && (
          <label className="flex items-center gap-2 mb-3 text-sm text-gray-200 cursor-pointer">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(e) => setDryRun(e.target.checked)}
              disabled={busy}
              className="h-4 w-4"
              data-test="breaker-dry-run-checkbox"
            />
            <span>{t('pricegap.breaker.dryRunCheckbox')}</span>
          </label>
        )}

        {/* Real-trip warning — prominent red banner. Only shown for test-fire
            when dry-run is unchecked. */}
        {showRealTripWarning && (
          <div
            className="mb-3 p-3 rounded border border-red-600 bg-red-900/30 text-red-300 text-sm"
            data-test="breaker-real-trip-warning"
          >
            {t('pricegap.breaker.realTripWarning')}
          </div>
        )}

        <p className="text-sm text-gray-300 mb-2" data-test="breaker-confirm-prompt">
          {t(promptKey)}
        </p>
        {!noPhrase && (
          <input
            type="text"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            placeholder={t('pricegap.breaker.confirmTypePlaceholder')}
            disabled={busy}
            autoFocus
            className="w-full bg-gray-900 border border-gray-600 rounded px-2 py-1 mb-3 text-white font-mono"
            data-test="breaker-confirm-input"
          />
        )}

        {errorMessage && (
          <p className="text-red-400 text-xs mb-3" data-test="breaker-confirm-error">
            {errorMessage}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            disabled={busy}
            className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white rounded text-sm disabled:opacity-50"
            onClick={() => {
              if (!busy) onClose();
            }}
            data-test="breaker-confirm-cancel"
          >
            {t('pricegap.breaker.confirmCancel')}
          </button>
          <button
            type="button"
            disabled={busy || !valid}
            className={`px-3 py-1.5 rounded text-sm text-white disabled:opacity-50 ${
              showRealTripWarning || destructiveSubmit
                ? 'bg-red-600 hover:bg-red-500'
                : 'bg-blue-600 hover:bg-blue-500'
            }`}
            onClick={() => onConfirm({ dryRun })}
            data-test="breaker-confirm-submit"
          >
            {t(resolvedSubmitKey)}
          </button>
        </div>
      </div>
    </div>
  );
};

export default BreakerConfirmModal;
