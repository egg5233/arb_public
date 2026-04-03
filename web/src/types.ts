export interface Opportunity {
  symbol: string;
  long_exchange: string;
  short_exchange: string;
  spread: number;         // bps per hour
  cost_ratio: number;
  oi_rank: number;
  score: number;
  long_rate: number;      // bps per hour
  short_rate: number;     // bps per hour
  interval_hours?: number;
  next_funding?: string;
  source?: string;        // "loris" or "coinglass"
  timestamp?: string;
}

export interface Position {
  id: string;
  symbol: string;
  long_exchange: string;
  short_exchange: string;
  long_size: number;
  short_size: number;
  long_entry: number;
  short_entry: number;
  long_exit: number;
  short_exit: number;
  status: string;
  entry_spread: number;       // bps per hour
  current_spread?: number;    // live bps per hour
  funding_collected: number;
  realized_pnl: number;
  created_at: string;
  updated_at: string;
  next_funding: string;
  rotation_pnl?: number;
  all_exchanges?: string[];
  last_rotated_from?: string;
  last_rotated_at?: string;
  rotation_count?: number;
  long_sl_order_id?: string;
  short_sl_order_id?: string;
  entry_fees?: number;
  exit_fees?: number;
  basis_gain_loss?: number;
  slippage?: number;
  exit_reason?: string;
  long_unrealized_pnl?: number;
  short_unrealized_pnl?: number;
  rotation_history?: RotationRecord[];
}

export interface RotationRecord {
  from: string;
  to: string;
  leg_side: string;
  pnl: number | null;
  timestamp: string;
}

export interface FundingEvent {
  exchange: string;
  side: string;
  amount: number;
  time: string;
}

export interface Stats {
  total_pnl: string;
  win_count: string;
  loss_count: string;
  trade_count: string;
}

export interface Alert {
  type: string;
  position_id: string;
  message: string;
  severity: string;
}

export interface ExchangeInfo {
  name: string;
  balance: number;
  spot_balance: number;
  account_type: string; // "unified" or "separate"
}

export interface RejectedOpportunity {
  symbol: string;
  long_exchange: string;
  short_exchange: string;
  spread: number;
  stage: string;   // "scanner", "verifier", "risk", "engine"
  reason: string;
  timestamp: string;
}

export interface LogEntry {
  timestamp: string;
  level: string;
  module: string;
  message: string;
}

export interface TransferRecord {
  id: string;
  from: string;
  to: string;
  coin: string;
  chain: string;
  amount: string;
  fee: string;
  tx_id: string;
  status: string;
  created_at: string;
}

export interface SpotPosition {
  id: string;
  symbol: string;
  exchange: string;
  direction: string;
  status: string;
  spot_size: number;
  spot_entry_price: number;
  spot_exit_price: number;
  futures_size: number;
  futures_entry: number;
  futures_exit: number;
  futures_side: string;
  borrow_amount: number;
  current_borrow_apr: number;
  borrow_cost_accrued: number;
  funding_apr: number;
  fee_pct: number;
  current_funding_apr: number;
  current_fee_pct: number;
  current_net_yield_apr: number;
  yield_data_source: string;
  yield_snapshot_at: string;
  notional_usdt: number;
  realized_pnl: number;
  entry_fees: number;
  exit_fees: number;
  exit_reason: string;
  peak_price_move_pct: number;
  margin_utilization_pct: number;
  created_at: string;
  updated_at: string;
}

export interface SpotStats {
  total_pnl: number;
  win_count: number;
  loss_count: number;
  trade_count: number;
}

export interface SpotOpportunity {
  symbol: string;
  base_coin: string;
  exchange: string;
  direction: string;
  funding_apr: number;
  borrow_apr: number;
  fee_pct: number;
  net_apr: number;
  source: string;
  timestamp: string;
  filter_status?: string;
}

export interface PriceGapResult {
  spot_bid: number;
  spot_ask: number;
  futures_bid: number;
  futures_ask: number;
  gap_pct: number;
  direction: string;
}

export interface LossLimitStatus {
  enabled: boolean;
  daily_loss: number;
  daily_limit: number;
  weekly_loss: number;
  weekly_limit: number;
  breached: boolean;
  breach_type: string; // "daily", "weekly", or ""
}

// ---------------------------------------------------------------------------
// Analytics types (Phase 4)
// ---------------------------------------------------------------------------

export interface PnLSnapshot {
  id: number;
  timestamp: number;
  strategy: string;
  exchange: string;
  cumulative_pnl: number;
  position_count: number;
  win_count: number;
  loss_count: number;
  funding_total: number;
  fees_total: number;
}

export interface StrategySummary {
  strategy: string;
  total_pnl: number;
  trade_count: number;
  win_count: number;
  loss_count: number;
  win_rate: number;
  apr: number;
  avg_hold_hours: number;
  funding_total: number;
  fees_total: number;
}

export interface ExchangeMetric {
  exchange: string;
  profit: number;
  trade_count: number;
  win_count: number;
  loss_count: number;
  win_rate: number;
  avg_slippage: number;
  apr: number;
}
