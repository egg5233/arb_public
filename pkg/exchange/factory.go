package exchange

// Factory logic moved to cmd/main.go to avoid import cycle.
// Sub-packages (binance, bitget, etc.) import this package for types/interfaces,
// so this package cannot import them back.
