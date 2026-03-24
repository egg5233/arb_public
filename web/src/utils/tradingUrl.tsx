export function tradingUrl(exchange: string, symbol: string): string {
  const base = symbol.replace(/USDT$/, '');
  const lc = base.toLowerCase();
  switch (exchange) {
    case 'binance':
      return `https://www.binance.com/futures/${symbol}`;
    case 'bybit':
      return `https://www.bybit.com/trade/usdt/${symbol}`;
    case 'gateio':
      return `https://www.gate.io/futures/usdt/${base}_USDT`;
    case 'bitget':
      return `https://www.bitget.com/futures/usdt/${symbol}`;
    case 'okx':
      return `https://www.okx.com/trade-swap/${lc}-usdt-swap`;
    case 'bingx':
      return `https://bingx.com/perpetual/${base}-USDT`;
    default:
      return '#';
  }
}

export function ExchangeLink({ exchange, symbol, className }: { exchange: string; symbol: string; className?: string }) {
  const url = tradingUrl(exchange, symbol);
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      className={`hover:underline ${className ?? ''}`}
      onClick={(e) => e.stopPropagation()}
    >
      {exchange}
    </a>
  );
}
