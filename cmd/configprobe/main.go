package main

import (
	"fmt"
	"os"

	"arb/internal/config"
)

func main() {
	if len(os.Args) > 1 {
		os.Setenv("CONFIG_FILE", os.Args[1])
	}
	c := config.Load()
	fmt.Println("--- AFTER Load() ---")
	dump(c)

	if len(os.Args) > 2 && os.Args[2] == "save" {
		fmt.Println("\n--- Calling SaveJSON() ---")
		if err := c.SaveJSON(); err != nil {
			fmt.Println("SaveJSON err:", err)
			return
		}
		c2 := config.Load()
		fmt.Println("\n--- AFTER reload ---")
		dump(c2)
	}
}

func dump(c *config.Config) {
	fmt.Printf("MaxPositions             = %d\n", c.MaxPositions)
	fmt.Printf("CapitalPerLeg            = %g\n", c.CapitalPerLeg)
	fmt.Printf("Leverage                 = %d\n", c.Leverage)
	fmt.Printf("MarginL3Threshold        = %g\n", c.MarginL3Threshold)
	fmt.Printf("EnableAnalytics          = %t\n", c.EnableAnalytics)
	fmt.Printf("PriceGapDebugLog         = %t\n", c.PriceGapDebugLog)
	fmt.Printf("PriceGapPaperMode        = %t\n", c.PriceGapPaperMode)
	fmt.Printf("SpotFuturesBacktestEnabled         = %t\n", c.SpotFuturesBacktestEnabled)
	fmt.Printf("SpotFuturesBacktestCoinGlassFallback = %t\n", c.SpotFuturesBacktestCoinGlassFallback)
	fmt.Printf("DelistFilterEnabled      = %t\n", c.DelistFilterEnabled)
	fmt.Printf("EnablePoolAllocator      = %t\n", c.EnablePoolAllocator)
	fmt.Printf("EnableSpreadReversal     = %t\n", c.EnableSpreadReversal)
	fmt.Printf("TradFiSigned             = %t\n", c.TradFiSigned)
	fmt.Printf("ScanMinutes              = %v\n", c.ScanMinutes)
}
