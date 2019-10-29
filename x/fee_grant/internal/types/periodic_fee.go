package types

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/fee_grant/exported"
)

// PeriodicFeeAllowance extends FeeAllowance to allow for both a maximum cap,
// as well as a limit per time period.
type PeriodicFeeAllowance struct {
	Basic BasicFeeAllowance

	// Period is the duration of one period
	Period Duration
	// PeriodSpendLimit is the maximum amount of tokens to be spent in this period
	PeriodSpendLimit sdk.Coins

	// PeriodCanSpend is how much is available until PeriodReset
	PeriodCanSpend sdk.Coins

	// PeriodRest is when the PeriodCanSpend is updated
	PeriodReset ExpiresAt
}

var _ exported.FeeAllowance = (*PeriodicFeeAllowance)(nil)

// Accept implements FeeAllowance and deducts the fees from the SpendLimit if possible
func (a *PeriodicFeeAllowance) Accept(fee sdk.Coins, blockTime time.Time, blockHeight int64) (remove bool, err error) {
	if a.Basic.Expiration.IsExpired(blockTime, blockHeight) {
		return true, ErrFeeLimitExpired()
	}

	a.TryResetPeriod(blockTime, blockHeight)

	// deduct from both the current period and the max amount
	var isNeg bool
	a.PeriodCanSpend, isNeg = a.PeriodCanSpend.SafeSub(fee)
	if isNeg {
		return false, ErrFeeLimitExceeded()
	}
	a.Basic.SpendLimit, isNeg = a.Basic.SpendLimit.SafeSub(fee)
	if isNeg {
		return false, ErrFeeLimitExceeded()
	}

	return a.Basic.SpendLimit.IsZero(), nil
}

// TryResetPeriod will reset the period if we hit the conditions
func (a *PeriodicFeeAllowance) TryResetPeriod(blockTime time.Time, blockHeight int64) {
	if !a.PeriodReset.IsZero() && !a.PeriodReset.IsExpired(blockTime, blockHeight) {
		return
	}
	// set CanSpend to the lesser of PeriodSpendLimit and the TotalLimit
	if _, isNeg := a.Basic.SpendLimit.SafeSub(a.PeriodSpendLimit); isNeg {
		a.PeriodCanSpend = a.Basic.SpendLimit
	} else {
		a.PeriodCanSpend = a.PeriodSpendLimit
	}

	// If we are within the period, step from expiration (eg. if you always do one tx per day, it will always reset the same time)
	// If we are more then one period out (eg. no activity in a week), reset is one period from this time
	a.PeriodReset = a.PeriodReset.MustStep(a.Period)
	if a.PeriodReset.IsExpired(blockTime, blockHeight) {
		a.PeriodReset = a.PeriodReset.FastForward(blockTime, blockHeight).MustStep(a.Period)
	}
}

// PrepareForExport adjusts all absolute block height (period reset, basic.expiration)
// with the dump height so they make sense after dump
func (a *PeriodicFeeAllowance) PrepareForExport(dumpTime time.Time, dumpHeight int64) exported.FeeAllowance {
	return &PeriodicFeeAllowance{
		Basic: BasicFeeAllowance{
			SpendLimit: a.Basic.SpendLimit,
			Expiration: a.Basic.Expiration.PrepareForExport(dumpTime, dumpHeight),
		},
		PeriodSpendLimit: a.PeriodSpendLimit,
		PeriodCanSpend:   a.PeriodCanSpend,
		Period:           a.Period,
		PeriodReset:      a.PeriodReset.PrepareForExport(dumpTime, dumpHeight),
	}
}

// ValidateBasic implements FeeAllowance and enforces basic sanity checks
func (a PeriodicFeeAllowance) ValidateBasic() error {
	if err := a.Basic.ValidateBasic(); err != nil {
		return err
	}

	if !a.PeriodSpendLimit.IsValid() {
		return sdk.ErrInvalidCoins("spend amount is invalid: " + a.PeriodSpendLimit.String())
	}
	if !a.PeriodSpendLimit.IsAllPositive() {
		return sdk.ErrInvalidCoins("spend limit must be positive")
	}
	if !a.PeriodCanSpend.IsValid() {
		return sdk.ErrInvalidCoins("can spend amount is invalid: " + a.PeriodCanSpend.String())
	}
	// We allow 0 for CanSpend
	if a.PeriodCanSpend.IsAnyNegative() {
		return sdk.ErrInvalidCoins("can spend must not be negative")
	}

	// TODO: ensure PeriodSpendLimit can be subtracted from total (same coin types)

	// check times
	if err := a.Period.ValidateBasic(); err != nil {
		return err
	}
	return a.PeriodReset.ValidateBasic()
}