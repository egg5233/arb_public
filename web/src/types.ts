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
