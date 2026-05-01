// Phase 15 Plan 15-05 — BreakerSubsection (PG-LIVE-02 frontend).
//
// Renders the Drawdown Circuit Breaker status card inside the Phase 14
// RampReconcileSection on the Pricegap-tracker tab. Phase 16 (PG-OPS-09)
// will lift this same component into a top-level Pricegap tab without
// restructuring it.
//
// Visible elements:
//   - Status badge (red Tripped / green Armed / gray Disabled or Paper)
//   - Realized 24h PnL + Threshold (heartbeat)
//   - When a trip exists: Last Trip Ts (Asia/Taipei) + Last Trip PnL +
//     Paused Candidate Count
//   - Recover button (visible only when Tripped)
//   - Test-fire button (always visible; disabled when Tripped — T-15-16)
//
// Mutators are routed through BreakerConfirmModal which enforces the
// case-sensitive typed-phrase gate (CONTEXT D-12 / T-15-13). Magic strings
// RECOVER + TEST-FIRE are LITERAL regardless of locale.
import { useState, type FC } from 'react';
import { useLocale } from '../../i18n/index.ts';
import { usePgBreaker } from '../../hooks/usePgBreaker.ts';
import { BreakerConfirmModal } from './BreakerConfirmModal.tsx';

// Format an Asia/Taipei wall-clock string from a unix-ms timestamp. Phase 8
// project convention: dashboard times displayed in Asia/Taipei (UTC+8).
function formatAsiaTaipei(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—';
  const d = new Date(ms);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString('sv-SE', { timeZone: 'Asia/Taipei' }).replace('T', ' ');
}

export const BreakerSubsection: FC = () => {
  const { t } = useLocale();
  const { state, recover, testFire, busy } = usePgBreaker();

  const [recoverOpen, setRecoverOpen] = useState(false);
  const [testFireOpen, setTestFireOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  if (!state) {
    return null; // Hook not seeded yet — parent widget shows its own skeleton.
  }

  // Badge color resolution per CONTEXT "Specifics":
  //   gray  → Disabled (cfg flag false)
  //   red   → Tripped
  //   green → Armed (live)
  let badgeClass = 'bg-gray-500 text-white';
  let badgeLabel = t('pricegap.breaker.disabled');
  if (state.enabled) {
    if (state.tripped) {
      badgeClass = 'bg-red-600 text-white';
      badgeLabel = t('pricegap.breaker.tripped');
    } else {
      badgeClass = 'bg-green-600 text-white';
      badgeLabel = t('pricegap.breaker.armed');
    }
  }

  const lastTrip = state.last_trip;

  const handleRecoverConfirm = async () => {
    setActionError(null);
    const res = await recover();
    if (!res.ok) {
      setActionError(res.error ?? 'recover failed');
      return;
    }
    setRecoverOpen(false);
  };

  const handleTestFireConfirm = async ({ dryRun }: { dryRun: boolean }) => {
    setActionError(null);
    const res = await testFire(dryRun);
    if (!res.ok) {
      setActionError(res.error ?? 'test-fire failed');
      return;
    }
    setTestFireOpen(false);
  };

  return (
    <div
      className="mt-6 pt-4 border-t border-gray-200"
      data-test="breaker-subsection"
    >
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-md font-semibold">{t('pricegap.breaker.status')}</h3>
        <span
          className={`inline-block px-3 py-1 rounded font-bold ${badgeClass}`}
          data-test="breaker-status-badge"
        >
          {badgeLabel}
        </span>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 text-sm">
        <div>
          <span className="text-gray-500">{t('pricegap.breaker.realized24hPnl')}:</span>{' '}
          <strong
            className={state.last_eval_pnl_usdt >= 0 ? 'text-green-700' : 'text-red-700'}
            data-test="breaker-24h-pnl"
          >
            ${state.last_eval_pnl_usdt.toFixed(2)}
          </strong>
        </div>
        <div>
          <span className="text-gray-500">{t('pricegap.breaker.threshold')}:</span>{' '}
          <strong data-test="breaker-threshold">
            ${state.threshold_usdt.toFixed(2)}
          </strong>
        </div>
        {lastTrip && (
          <>
            <div>
              <span className="text-gray-500">{t('pricegap.breaker.lastTripTs')}:</span>{' '}
              <span data-test="breaker-last-trip-ts">
                {formatAsiaTaipei(lastTrip.trip_ts_ms)}
              </span>
            </div>
            <div>
              <span className="text-gray-500">{t('pricegap.breaker.lastTripPnl')}:</span>{' '}
              <strong
                className={lastTrip.trip_pnl_usdt >= 0 ? 'text-green-700' : 'text-red-700'}
                data-test="breaker-last-trip-pnl"
              >
                ${lastTrip.trip_pnl_usdt.toFixed(2)}
              </strong>
            </div>
            <div>
              <span className="text-gray-500">{t('pricegap.breaker.pausedCount')}:</span>{' '}
              <span data-test="breaker-paused-count">{lastTrip.paused_candidate_count}</span>
            </div>
          </>
        )}
      </div>

      <div className="mt-3 flex items-center gap-3">
        {state.tripped && (
          <button
            type="button"
            disabled={busy}
            onClick={() => {
              setActionError(null);
              setRecoverOpen(true);
            }}
            className="px-3 py-1.5 bg-yellow-600 hover:bg-yellow-500 text-white rounded text-sm disabled:opacity-50"
            data-test="breaker-recover-button"
          >
            {t('pricegap.breaker.recoverButton')}
          </button>
        )}
        <button
          type="button"
          disabled={busy || state.tripped}
          onClick={() => {
            setActionError(null);
            setTestFireOpen(true);
          }}
          className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded text-sm disabled:opacity-50"
          data-test="breaker-test-fire-button"
        >
          {t('pricegap.breaker.testFireButton')}
        </button>
      </div>

      {state.tripped && (
        <p className="mt-2 text-xs text-gray-500" data-test="breaker-recovery-instruction">
          {t('pricegap.breaker.recoveryInstruction')}
        </p>
      )}

      <BreakerConfirmModal
        open={recoverOpen}
        action="recover"
        magicPhrase="RECOVER"
        promptKey="pricegap.breaker.confirmRecoverPrompt"
        busy={busy}
        errorMessage={actionError}
        onClose={() => {
          if (!busy) {
            setRecoverOpen(false);
            setActionError(null);
          }
        }}
        onConfirm={() => void handleRecoverConfirm()}
      />

      <BreakerConfirmModal
        open={testFireOpen}
        action="test-fire"
        magicPhrase="TEST-FIRE"
        promptKey="pricegap.breaker.confirmTestFirePrompt"
        busy={busy}
        errorMessage={actionError}
        onClose={() => {
          if (!busy) {
            setTestFireOpen(false);
            setActionError(null);
          }
        }}
        onConfirm={(opts) => void handleTestFireConfirm(opts)}
      />
    </div>
  );
};

export default BreakerSubsection;
