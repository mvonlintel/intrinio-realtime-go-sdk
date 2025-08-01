package composite

import (
	"log"
	"math"
	"time"
	"github.com/intrinio/intrinio-realtime-go-sdk"
)

// BlackScholesGreekCalculator provides static methods for calculating Black-Scholes Greeks
type BlackScholesGreekCalculator struct{}

const (
	lowVol       = 0.0
	highVol      = 5.0
	volTolerance = 0.0001
	minZScore    = -8.0
	maxZScore    = 8.0
	rootPi       = 4.44288293816 //math.Sqrt(2.0 * math.Pi)
)

// Calculate calculates the Black-Scholes Greeks for an options contract
func (b *BlackScholesGreekCalculator) Calculate(riskFreeInterestRate, dividendYield float64,
	underlyingTrade *intrinio.EquityTrade, latestOptionTrade *intrinio.OptionTrade, latestOptionQuote *intrinio.OptionQuote) Greek {
	
	log.Printf("riskFreeInterestRate: %f, dividendYield: %f", riskFreeInterestRate, dividendYield)
	log.Printf("underlyingTrade: %+v", underlyingTrade)
	log.Printf("latestOptionTrade: %+v", latestOptionTrade)
	log.Printf("latestOptionQuote: %+v", latestOptionQuote)
	log.Printf("latestOptionQuote.AskPrice: %f, latestOptionQuote.BidPrice: %f", latestOptionQuote.AskPrice, latestOptionQuote.BidPrice)

	if latestOptionQuote.AskPrice <= 0.0 || latestOptionQuote.BidPrice <= 0.0 || 
		riskFreeInterestRate <= 0.0 || underlyingTrade.Price <= 0.0 {
		return NewGreek(0.0, 0.0, 0.0, 0.0, 0.0, false)
	}
	
	yearsToExpiration := b.getYearsToExpiration(latestOptionTrade, latestOptionQuote)
	underlyingPrice := float64(underlyingTrade.Price)
	strike := float64(b.getStrikePrice(latestOptionTrade.ContractId))
	isPut := b.isPut(latestOptionTrade.ContractId)
	marketPrice := float64((latestOptionQuote.AskPrice + latestOptionQuote.BidPrice) / 2.0)
	
	if yearsToExpiration <= 0.0 || strike <= 0.0 {
		return NewGreek(0.0, 0.0, 0.0, 0.0, 0.0, false)
	}
	
	impliedVolatility := b.calcImpliedVolatility(isPut, underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice)
	if impliedVolatility == 0.0 {
		return NewGreek(0.0, 0.0, 0.0, 0.0, 0.0, false)
	}
	
	delta := b.calcDelta(isPut, underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, impliedVolatility)
	gamma := b.calcGamma(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, impliedVolatility)
	theta := b.calcTheta(isPut, underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, impliedVolatility)
	vega := b.calcVega(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, impliedVolatility)
	
	return NewGreek(impliedVolatility, delta, gamma, theta, vega, true)
}

// calcImpliedVolatility calculates the implied volatility
func (b *BlackScholesGreekCalculator) calcImpliedVolatility(isPut bool, underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice float64) float64 {
	if isPut {
		return b.calcImpliedVolatilityPut(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice)
	}
	return b.calcImpliedVolatilityCall(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice)
}

// calcImpliedVolatilityCall calculates implied volatility for call options
func (b *BlackScholesGreekCalculator) calcImpliedVolatilityCall(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice float64) float64 {
	low := lowVol
	high := highVol
	
	for (high - low) > volTolerance {
		mid := (high + low) / 2.0
		if b.calcPriceCall(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, mid, dividendYield) > float64(marketPrice) {
			high = mid
		} else {
			low = mid
		}
	}
	
	return (high + low) / 2.0
}

// calcImpliedVolatilityPut calculates implied volatility for put options
func (b *BlackScholesGreekCalculator) calcImpliedVolatilityPut(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice float64) float64 {
	low := lowVol
	high := highVol
	
	for (high - low) > volTolerance {
		mid := (high + low) / 2.0
		if b.calcPricePut(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, mid, dividendYield) > float64(marketPrice) {
			high = mid
		} else {
			low = mid
		}
	}
	
	return (high + low) / 2.0
}

// calcDelta calculates the delta
func (b *BlackScholesGreekCalculator) calcDelta(isPut bool, underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	if isPut {
		return b.calcDeltaPut(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, sigma)
	}
	return b.calcDeltaCall(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, sigma)
}

// calcDeltaCall calculates delta for call options
func (b *BlackScholesGreekCalculator) calcDeltaCall(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	return b.normalSDist(b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield))
}

// calcDeltaPut calculates delta for put options
func (b *BlackScholesGreekCalculator) calcDeltaPut(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	return b.calcDeltaCall(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, sigma) - 1.0
}

// calcGamma calculates the gamma
func (b *BlackScholesGreekCalculator) calcGamma(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	d1 := b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	return b.phi(d1) * math.Exp(-dividendYield*yearsToExpiration) / (underlyingPrice * sigma * math.Sqrt(yearsToExpiration))
}

// calcTheta calculates the theta
func (b *BlackScholesGreekCalculator) calcTheta(isPut bool, underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	if isPut {
		return b.calcThetaPut(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, sigma)
	}
	return b.calcThetaCall(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, dividendYield, marketPrice, sigma)
}

// calcThetaCall calculates theta for call options
func (b *BlackScholesGreekCalculator) calcThetaCall(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	d1 := b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	d2 := b.d2(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	
	return -(underlyingPrice * sigma * b.phi(d1) * math.Exp(-dividendYield*yearsToExpiration)) / (2 * math.Sqrt(yearsToExpiration)) -
		riskFreeInterestRate * strike * math.Exp(-riskFreeInterestRate*yearsToExpiration) * b.normalSDist(d2) +
		dividendYield * underlyingPrice * math.Exp(-dividendYield*yearsToExpiration) * b.normalSDist(d1)
}

// calcThetaPut calculates theta for put options
func (b *BlackScholesGreekCalculator) calcThetaPut(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	d1 := b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	d2 := b.d2(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	
	return -(underlyingPrice * sigma * b.phi(d1) * math.Exp(-dividendYield*yearsToExpiration)) / (2 * math.Sqrt(yearsToExpiration)) +
		riskFreeInterestRate * strike * math.Exp(-riskFreeInterestRate*yearsToExpiration) * b.normalSDist(-d2) -
		dividendYield * underlyingPrice * math.Exp(-dividendYield*yearsToExpiration) * b.normalSDist(-d1)
}

// calcVega calculates the vega
func (b *BlackScholesGreekCalculator) calcVega(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate, dividendYield, marketPrice, sigma float64) float64 {
	d1 := b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	return underlyingPrice * math.Exp(-dividendYield*yearsToExpiration) * b.phi(d1) * math.Sqrt(yearsToExpiration) / 100.0
}

// d1 calculates the d1 parameter
func (b *BlackScholesGreekCalculator) d1(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate float64, sigma float64, dividendYield float64) float64 {
	return (math.Log(underlyingPrice/strike) + (riskFreeInterestRate-dividendYield+0.5*sigma*sigma)*yearsToExpiration) / (sigma * math.Sqrt(yearsToExpiration))
}

// d2 calculates the d2 parameter
func (b *BlackScholesGreekCalculator) d2(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate float64, sigma float64, dividendYield float64) float64 {
	return b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield) - sigma*math.Sqrt(yearsToExpiration)
}

// normalSDist calculates the cumulative normal distribution
func (b *BlackScholesGreekCalculator) normalSDist(z float64) float64 {
	if z < minZScore {
		return 0.0
	}
	if z > maxZScore {
		return 1.0
	}
	
	if z >= 0.0 {
		return 0.5 + 0.5*b.phi(z)
	}
	return 0.5 - 0.5*b.phi(-z)
}

// phi calculates the normal probability density function
func (b *BlackScholesGreekCalculator) phi(x float64) float64 {
	return math.Exp(-0.5*x*x) / rootPi
}

// calcPriceCall calculates the Black-Scholes price for call options
func (b *BlackScholesGreekCalculator) calcPriceCall(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate float64, sigma float64, dividendYield float64) float64 {
	d1 := b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	d2 := b.d2(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	
	return underlyingPrice*math.Exp(-dividendYield*yearsToExpiration)*b.normalSDist(d1) -
		strike*math.Exp(-riskFreeInterestRate*yearsToExpiration)*b.normalSDist(d2)
}

// calcPricePut calculates the Black-Scholes price for put options
func (b *BlackScholesGreekCalculator) calcPricePut(underlyingPrice, strike float64, yearsToExpiration float64, riskFreeInterestRate float64, sigma float64, dividendYield float64) float64 {
	d1 := b.d1(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	d2 := b.d2(underlyingPrice, strike, yearsToExpiration, riskFreeInterestRate, sigma, dividendYield)
	
	return strike*math.Exp(-riskFreeInterestRate*yearsToExpiration)*b.normalSDist(-d2) -
		underlyingPrice*math.Exp(-dividendYield*yearsToExpiration)*b.normalSDist(-d1)
}

// getYearsToExpiration calculates the years to expiration
func (b *BlackScholesGreekCalculator) getYearsToExpiration(latestOptionTrade *intrinio.OptionTrade, latestOptionQuote *intrinio.OptionQuote) float64 {
	// Use the expiration date from the contract
	expirationDate := b.getExpirationDate(latestOptionTrade.ContractId)
	now := time.Now()
	
	yearsToExpiration := expirationDate.Sub(now).Hours() / (365.0 * 24.0)
	if yearsToExpiration < 0.0 {
		return 0.0
	}
	return yearsToExpiration
}

// getStrikePrice extracts the strike price from the contract identifier
func (b *BlackScholesGreekCalculator) getStrikePrice(contract string) float64{
	if len(contract) < 19 {
		return 0.0
	}
	
	// Extract strike price from contract (format: AAPL__201016C00100000)
	strikeStr := contract[13:19]
	
	var whole uint32
	for i := 0; i < 5; i++ {
		whole += uint32(strikeStr[i]-'0') * uint32(math.Pow10(4-i))
	}
	
	part := float64(strikeStr[5]-'0') * 0.1
	
	return float64(whole) + part
}

// isPut checks if the option is a put
func (b *BlackScholesGreekCalculator) isPut(contract string) bool {
	if len(contract) < 13 {
		return false
	}
	return contract[12] == 'P'
}

// getExpirationDate extracts the expiration date from the contract identifier
func (b *BlackScholesGreekCalculator) getExpirationDate(contract string) time.Time {
	if len(contract) < 12 {
		return time.Time{}
	}
	
	// Extract date from contract (format: AAPL__201016C00100000)
	dateStr := contract[6:12]
	
	// Parse date in format "yyMMdd"
	expirationDate, err := time.Parse("060102", dateStr)
	if err != nil {
		return time.Time{}
	}
	
	return expirationDate
} 