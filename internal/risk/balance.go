package risk

import "arb/pkg/exchange"

// EffectiveOrderAvailable returns the conservative balance usable for opening
// new derivatives orders. Some unified accounts can report a raw Available
// balance that is larger than the exchange's own transferable/collateral
// headroom while positions are open; use the lower authoritative cap when it
// is present.
func EffectiveOrderAvailable(bal *exchange.Balance) float64 {
	if bal == nil {
		return 0
	}
	available := bal.Available
	if bal.MaxTransferOutAuthoritative {
		if bal.MaxTransferOut < available {
			available = bal.MaxTransferOut
		}
	} else if bal.MaxTransferOut > 0 && bal.MaxTransferOut < available {
		available = bal.MaxTransferOut
	}
	if available < 0 {
		return 0
	}
	return available
}
