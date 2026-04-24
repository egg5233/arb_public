package main

import (
	"fmt"
	"math"
)

func main() {
	// Data from logs 2026-04-11 14:45 UTC
	type exchData struct {
		futures, spot, futuresTotal, marginRatio float64
		isUnified, hasPos                        bool
	}

	balances := map[string]exchData{
		"okx":    {futures: 64.48, spot: 0, futuresTotal: 111.07, marginRatio: 0.0420, isUnified: false, hasPos: true},
		"gateio": {futures: 15.33, spot: 0, futuresTotal: 59.27, marginRatio: 0.1769, isUnified: false, hasPos: true},
	}

	needs := map[string]float64{
		"okx":    59.98,
		"gateio": 59.98,
	}

	L4 := 0.80
	marginEpsilon := 0.005

	// Test both MSM values
	for _, msm := range []float64{1.2, 2.0} {
		fmt.Printf("\n=== MarginSafetyMultiplier = %.1f ===\n", msm)

		for _, name := range []string{"gateio", "okx"} {
			bal := balances[name]
			need := needs[name]
			targetFreeRatio := 1 - L4

			fmt.Printf("\n--- %s ---\n", name)
			fmt.Printf("  need=%.2f futures=%.2f spot=%.2f total=%.2f\n", need, bal.futures, bal.spot, bal.futuresTotal)

			if bal.futures >= need {
				fmt.Printf("  futures >= need → 'sufficient' branch\n")

				// Check spot→futures path
				if !bal.isUnified && bal.spot > 0 && bal.futuresTotal > 0 {
					projectedAvail := bal.futures - need
					if projectedAvail < 0 {
						projectedAvail = 0
					}
					projectedRatio := 1 - projectedAvail/bal.futuresTotal
					fmt.Printf("  spot>0 path: projRatio=%.4f\n", projectedRatio)
					if projectedRatio >= L4 {
						extra := (need - bal.futuresTotal*targetFreeRatio) / targetFreeRatio
						if extra > bal.spot {
							extra = bal.spot
						}
						fmt.Printf("  would do spot→futures: extra=%.2f\n", extra)
					}
					fmt.Printf("  → continue (skip L4 check)\n")
					continue // this is what the code does
				}
				fmt.Printf("  spot=0 or unified → skip spot→futures, fall through to L4 check\n")

				// L4 check
				if bal.futuresTotal > 0 {
					actualMargin := need / msm
					if actualMargin <= 0 {
						actualMargin = need
					}
					projectedAvail := bal.futures - actualMargin
					if projectedAvail < 0 {
						projectedAvail = 0
					}
					projectedRatio := 1 - projectedAvail/bal.futuresTotal
					fmt.Printf("  L4 check: actualMargin=%.2f projAvail=%.2f projRatio=%.4f L4=%.4f\n",
						actualMargin, projectedAvail, projectedRatio, L4)

					if projectedRatio >= L4 {
						targetRatio := L4 - marginEpsilon
						freeTarget := 1.0 - targetRatio
						ratioDeficit := (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
						if ratioDeficit < 0 {
							ratioDeficit = 0
						}
						fmt.Printf("  ⚠️  WOULD QUEUE crossDeficit: ratioDeficit=%.2f\n", ratioDeficit)
					} else {
						fmt.Printf("  ✓  projRatio < L4, no crossDeficit needed\n")
					}
				}
			} else {
				fmt.Printf("  futures < need → 'deficit' branch\n")
				marginDeficit := need - bal.futures
				actualMargin := need / msm
				targetRatio := L4 - marginEpsilon
				freeTarget := 1.0 - targetRatio
				var ratioDeficit float64
				if freeTarget > 0 && bal.futuresTotal > 0 {
					ratioDeficit = (freeTarget*bal.futuresTotal - bal.futures + actualMargin) / targetRatio
					if ratioDeficit < 0 {
						ratioDeficit = 0
					}
				}
				transferAmt := math.Max(marginDeficit, ratioDeficit)
				fmt.Printf("  marginDef=%.2f ratioDef=%.2f transferAmt=%.2f\n", marginDeficit, ratioDeficit, transferAmt)
			}
		}

		// Also simulate real entry L4 check (manager.go:458-468)
		fmt.Printf("\n--- Real entry L4 check (okx, lev=3) ---\n")
		size := 5050.0
		price := 0.029645
		lev := 3.0
		longMarginPerLeg := size * price / lev
		okxAvail := 64.33
		okxTotal := 110.77
		projAvail := okxAvail - longMarginPerLeg
		if projAvail < 0 {
			projAvail = 0
		}
		projRatio := 1 - projAvail/okxTotal
		fmt.Printf("  longMarginPerLeg=%.2f okxAvail=%.2f okxTotal=%.2f\n", longMarginPerLeg, okxAvail, okxTotal)
		fmt.Printf("  projAvail=%.2f projRatio=%.4f L4=%.4f → %s\n",
			projAvail, projRatio, L4, map[bool]string{true: "REJECTED", false: "OK"}[projRatio >= L4])
	}
}
