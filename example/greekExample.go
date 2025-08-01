package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/intrinio/intrinio-realtime-go-sdk"
	"github.com/intrinio/intrinio-realtime-go-sdk/composite"
)

// GreekSampleApp demonstrates real-time Greek calculations
type GreekSampleApp struct {
	timer                    *time.Ticker
	greekClient              *composite.GreekClient
	dataCache                composite.DataCache
	seenGreekTickers         map[string]string
	seenGreekTickersMutex    sync.RWMutex
	
	// Event counters
	optionsTradeEventCount   uint64
	optionsQuoteEventCount   uint64
	equitiesTradeEventCount  uint64
	equitiesQuoteEventCount  uint64
	greekUpdatedEventCount   uint64
	
	// Mock data generation
	stopChan                 chan bool
	wg                       sync.WaitGroup
}

// NewGreekSampleApp creates a new Greek sample app
func NewGreekSampleApp() *GreekSampleApp {
	return &GreekSampleApp{
		seenGreekTickers: make(map[string]string),
		stopChan:         make(chan bool),
	}
}

// OnOptionsQuote handles options quote updates
func (g *GreekSampleApp) OnOptionsQuote(quote intrinio.OptionQuote) {
	atomic.AddUint64(&g.optionsQuoteEventCount, 1)
	g.dataCache.SetOptionsQuote(&quote)
}

// OnOptionsTrade handles options trade updates
func (g *GreekSampleApp) OnOptionsTrade(trade intrinio.OptionTrade) {
	atomic.AddUint64(&g.optionsTradeEventCount, 1)
	g.dataCache.SetOptionsTrade(&trade)
}

// OnEquitiesQuote handles equities quote updates
func (g *GreekSampleApp) OnEquitiesQuote(quote intrinio.EquityQuote) {
	atomic.AddUint64(&g.equitiesQuoteEventCount, 1)
	g.dataCache.SetEquityQuote(&quote)

	// For demonstration and testing greeks, inject a mock options trade and quote for AAPL
	aaplTrade := intrinio.OptionTrade(intrinio.OptionTrade{
		ContractId: "AAPL__250808C00200000",
		Price:    	2.26,
		Size:     	2,
		Timestamp:  float64(time.Now().Unix()),
	})
	g.dataCache.SetOptionsTrade(&aaplTrade)

	aaplQuote := intrinio.OptionQuote(intrinio.OptionQuote{
		ContractId: "AAPL__250808C00200000",
		AskPrice:    	2.28,
		AskSize:     	2,
		BidPrice:		2.24,
		BidSize:		3,
		Timestamp:  float64(time.Now().Unix()),
	})
	g.dataCache.SetOptionsQuote(&aaplQuote)
}

// OnEquitiesTrade handles equities trade updates
func (g *GreekSampleApp) OnEquitiesTrade(trade intrinio.EquityTrade) {
	atomic.AddUint64(&g.equitiesTradeEventCount, 1)
	g.dataCache.SetEquityTrade(&trade)
}

// OnGreek handles Greek calculation updates
func (g *GreekSampleApp) OnGreek(key string, datum *float64, optionsContractData composite.OptionsContractData, securityData composite.SecurityData, dataCache composite.DataCache) {
	atomic.AddUint64(&g.greekUpdatedEventCount, 1)
	
	g.seenGreekTickersMutex.Lock()
	g.seenGreekTickers[securityData.GetTickerSymbol()] = optionsContractData.GetContract()
	g.seenGreekTickersMutex.Unlock()
	
	log.Printf("Greek: %s\t\t%s\t\t%f", optionsContractData.GetContract(), key, *datum)
}

// timerCallback prints statistics every minute
func (g *GreekSampleApp) timerCallback() {
	log.Printf("=== Statistics Update ===")
	log.Printf("Options Trade Events: %d", atomic.LoadUint64(&g.optionsTradeEventCount))
	log.Printf("Options Quote Events: %d", atomic.LoadUint64(&g.optionsQuoteEventCount))
	log.Printf("Equities Trade Events: %d", atomic.LoadUint64(&g.equitiesTradeEventCount))
	log.Printf("Equities Quote Events: %d", atomic.LoadUint64(&g.equitiesQuoteEventCount))
	log.Printf("Greek Updates: %d", atomic.LoadUint64(&g.greekUpdatedEventCount))
	
	allSecurityData := g.dataCache.GetAllSecurityData()
	log.Printf("Data Cache Security Count: %d", len(allSecurityData))
	
	// Count securities with dividend yield
	dividendYieldCount := 0
	for _, securityData := range allSecurityData {
		if securityData.GetSupplementaryDatum("DividendYield") != nil {
			dividendYieldCount++
		}
	}
	log.Printf("Dividend Yield Count: %d", dividendYieldCount)
	
	g.seenGreekTickersMutex.RLock()
	uniqueSecuritiesCount := len(g.seenGreekTickers)
	g.seenGreekTickersMutex.RUnlock()
	log.Printf("Unique Securities with Greeks Count: %d", uniqueSecuritiesCount)
	log.Printf("=== End Statistics ===\n")
}

// Run starts the Greek sample app
func (g *GreekSampleApp) runGreekExample() error {
	log.Println("Starting Greek sample app")

	symbols := []string{"AAPL", "MSFT", "SPY", "QQQ"}

	var equitiesConfig intrinio.Config = intrinio.LoadConfig("equities-config.json")
	var equitiesClient *intrinio.Client = intrinio.NewEquitiesClient(equitiesConfig, g.OnEquitiesTrade, g.OnEquitiesQuote)
	
	equitiesClient.Start()
	equitiesClient.JoinMany(symbols)

	var optionsConfig intrinio.Config = intrinio.LoadConfig("options-config.json")
	var optionsClient *intrinio.Client = intrinio.NewOptionsClient(optionsConfig, g.OnOptionsTrade, g.OnOptionsQuote, nil, nil)
	
	optionsClient.Start()
	optionsClient.JoinMany(symbols)

	// Create data cache
	g.dataCache = composite.NewDataCache()
	
	// Set up Greek update frequency
	updateFrequency := composite.EveryDividendYieldUpdate |
		composite.EveryRiskFreeInterestRateUpdate |
		composite.EveryOptionsTradeUpdate |
		composite.EveryEquityTradeUpdate
	
	// Create Greek client
	g.greekClient = composite.NewGreekClient(updateFrequency, g.OnGreek, optionsConfig.ApiKey, g.dataCache)
	g.greekClient.AddBlackScholes()
	g.greekClient.Start()
	
	// Set initial risk-free interest rate
	initialRate := 0.025 // 2.5%
	g.dataCache.SetSupplementaryDatum("RiskFreeInterestRate", &initialRate, func(key string, oldValue, newValue *float64) *float64 {
		return newValue
	})
	
	// Set initial dividend yields for some tickers
	aaplDividend := 0.0051 
	googlDividend := 0.0044  
	msftDividend := 0.0063 
	spyDividend := 0.0113
	qqqDividend := 0.0052
	
	aaplSecurity := composite.NewSecurityData("AAPL")
	aaplSecurity.SetSupplementaryDatum("DividendYield", &aaplDividend, func(key string, oldValue, newValue *float64) *float64 {
		return newValue
	})
	
	googlSecurity := composite.NewSecurityData("GOOGL")
	googlSecurity.SetSupplementaryDatum("DividendYield", &googlDividend, func(key string, oldValue, newValue *float64) *float64 {
		return newValue
	})
	
	msftSecurity := composite.NewSecurityData("MSFT")
	msftSecurity.SetSupplementaryDatum("DividendYield", &msftDividend, func(key string, oldValue, newValue *float64) *float64 {
		return newValue
	})

	spySecurity := composite.NewSecurityData("SPY")
	spySecurity.SetSupplementaryDatum("DividendYield", &spyDividend, func(key string, oldValue, newValue *float64) *float64 {
		return newValue
	})

	qqqSecurity := composite.NewSecurityData("QQQ")
	qqqSecurity.SetSupplementaryDatum("DividendYield", &qqqDividend, func(key string, oldValue, newValue *float64) *float64 {
		return newValue
	})

	// Start statistics timer
	g.timer = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-g.stopChan:
				return
			case <-g.timer.C:
				g.timerCallback()
			}
		}
	}()
	
	// Wait for interrupt signal
	log.Println("Greek sample app running. Press Ctrl+C to stop.")
	
	// Keep the app running
	select {
	case <-g.stopChan:
		break
	}
	
	return nil
}

// Stop stops the Greek sample app
func (g *GreekSampleApp) Stop() {
	log.Println("Stopping Greek sample app")
	
	// Stop the timer
	if g.timer != nil {
		g.timer.Stop()
	}
	
	// Stop mock data generation
	close(g.stopChan)
	
	// Stop Greek client
	if g.greekClient != nil {
		g.greekClient.Stop()
	}
	
	// Wait for all goroutines to finish
	g.wg.Wait()
	
	log.Println("Greek sample app stopped")
}

// Helper function to extract ticker symbol from contract
func extractTickerFromContract(contract string) string {
	if len(contract) < 6 {
		return ""
	}
	
	// Find the first underscore sequence
	for i := 0; i < len(contract)-1; i++ {
		if contract[i] == '_' && contract[i+1] == '_' {
			return contract[:i]
		}
	}
	
	// Fallback: take first 6 characters
	if len(contract) >= 6 {
		return contract[:6]
	}
	
	return ""
}