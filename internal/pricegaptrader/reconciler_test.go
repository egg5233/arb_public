package pricegaptrader

import "testing"

// Phase 14 reconciler stubs — bodies land in plan 14-02.
// pg:reconcile:daily byte-equality (D-04), ExchangeClosedAt fallback (D-10),
// 3-retry shape (Q1), anomaly flagging (D-09).

func TestReconcile_Idempotency_ByteEqual(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-02")
}

func TestReconcile_MissingExchangeCloseTs_FallsBackToLocal(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-02")
}

func TestReconcile_TripleFail_SkipsDay(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-02")
}

func TestReconcile_AnomalyHighSlippage_Flagged(t *testing.T) {
	t.Skip("Wave 0 stub: implementation lands in plan 14-02")
}
