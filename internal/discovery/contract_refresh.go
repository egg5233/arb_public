package discovery

import (
	"time"

	"arb/pkg/exchange"
)

// defaultContractRefreshInterval is the fallback poll cadence when the
// config field is unset or zero. 1h gives at most a 1h exposure window
// between a Binance/Bybit delist announcement (which updates deliveryDate
// at announcement time per their docs) and the symbol being blacklisted.
const defaultContractRefreshInterval = 1 * time.Hour

// deliveryDateLookahead bounds how far in the future a contract's
// deliveryDate must fall to be treated as a real upcoming delist. This
// guards against any exchange returning very-far-future timestamps that
// somehow slip past the adapter's year-2099 sentinel cutoff.
const deliveryDateLookahead = 365 * 24 * time.Hour

// StartContractRefresh launches a background poller that periodically
// refreshes per-exchange contract metadata via LoadAllContracts and writes
// any deliveryDate-flagged perpetuals to the existing arb:delist:{SYMBOL}
// Redis blacklist. Uses the existing scanner.SetDelistCooldown writer so all
// downstream consumers (scanner step-8 filter, engine.checkDelistPositions,
// spot-engine risk gate) work unchanged.
//
// The poller is the primary delist signal — the article scraper at delist.go
// remains as a belt-and-suspenders second source. When both write the same
// key, the operation is idempotent (same date, same key).
func (s *Scanner) StartContractRefresh() {
	go func() {
		// Immediate poll on startup so a freshly-restarted bot picks up any
		// delists announced while it was down.
		s.refreshContractsAndFlagDelists()

		interval := s.contractRefreshInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.refreshContractsAndFlagDelists()
			case <-s.stopCh:
				s.log.Info("contract refresh stopped")
				return
			}
		}
	}()
}

// contractRefreshInterval returns the configured cadence with a sensible
// default when unset.
func (s *Scanner) contractRefreshInterval() time.Duration {
	if s.cfg != nil && s.cfg.ContractRefreshInterval > 0 {
		return s.cfg.ContractRefreshInterval
	}
	return defaultContractRefreshInterval
}

// refreshContractsAndFlagDelists is the body of the periodic poller. It
// re-loads contract metadata for every configured exchange and writes
// blacklist entries for any contract whose DeliveryDate is set to a real
// near-future timestamp.
func (s *Scanner) refreshContractsAndFlagDelists() {
	now := time.Now().UTC()
	flaggedTotal := 0
	newTotal := 0

	for exchName, exch := range s.exchanges {
		contracts, err := exch.LoadAllContracts()
		if err != nil {
			s.log.Warn("contract refresh: LoadAllContracts(%s) failed: %v", exchName, err)
			continue
		}

		// Refresh the in-memory cache so other parts of the scanner see
		// the latest contract metadata. We update only this exchange's
		// slot to avoid clobbering data from exchanges whose poll failed
		// in this tick.
		s.replaceContractsForExchange(exchName, contracts)

		exchFlagged := 0
		exchNew := 0
		for _, ci := range contracts {
			if ci.DeliveryDate.IsZero() {
				continue
			}
			// Only flag contracts whose delivery is near-term. The adapter
			// already filters the year-2100 sentinel; this is a second guard
			// in case any exchange invents a different far-future marker.
			if !ci.DeliveryDate.Before(now.Add(deliveryDateLookahead)) {
				continue
			}

			exchFlagged++
			flaggedTotal++

			key := delistRedisPrefix + ci.Symbol
			existing, _ := s.db.Get(key)

			// TTL: until delist date + 7d buffer (matches existing
			// delistBufferDays from delist.go), with a 1h floor so an
			// already-past delist still gets a meaningful blacklist window.
			ttl := time.Until(ci.DeliveryDate) + time.Duration(delistBufferDays)*24*time.Hour
			if ttl < time.Hour {
				ttl = time.Hour
			}

			dateStr := ci.DeliveryDate.Format("2006-01-02")
			if err := s.db.SetWithTTL(key, dateStr, ttl); err != nil {
				s.log.Error("contract refresh: failed to write Redis key %s: %v", key, err)
				continue
			}

			if existing == "" {
				exchNew++
				newTotal++
				s.log.Warn("delivery-date delist detected: %s on %s scheduled for %s (in %s)",
					ci.Symbol, exchName, dateStr, time.Until(ci.DeliveryDate).Round(time.Minute))
			}
		}

		s.log.Info("contract refresh: %s — %d contracts loaded, %d delisting, %d newly blacklisted",
			exchName, len(contracts), exchFlagged, exchNew)
	}

	if flaggedTotal == 0 {
		s.log.Info("contract refresh: poll complete, no delivery-date delists detected")
	} else {
		s.log.Info("contract refresh: poll complete, %d delisting symbols seen, %d newly blacklisted", flaggedTotal, newTotal)
	}
}

// replaceContractsForExchange swaps the contract map for a single exchange
// in the scanner's in-memory cache without disturbing other exchanges. Used
// by the periodic refresh so the cache reflects the latest LoadAllContracts
// output (including DeliveryDate flags) for downstream consumers.
func (s *Scanner) replaceContractsForExchange(exchName string, contracts map[string]exchange.ContractInfo) {
	s.contractsMu.Lock()
	defer s.contractsMu.Unlock()
	if s.contracts == nil {
		s.contracts = make(map[string]map[string]exchange.ContractInfo)
	}
	s.contracts[exchName] = contracts
}
