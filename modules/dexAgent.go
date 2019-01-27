package modules

import (
	"fmt"
	"math"
	"strconv"

	"github.com/interstellar/kelp/model"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/pkg/errors"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

const baseReserve = 0.5
const baseFee = 0.0000100
const maxLumenTrust = math.MaxFloat64
const maxPageLimit = 200

// DexAgent manages all transactions with the Stellar DEX
type DexAgent struct {
	API               *horizon.Client
	SourceSeed        string
	TradingSeed       string
	SourceAccount     string
	TradingAccount    string
	Network           build.Network
	threadTracker     *multithreading.ThreadTracker
	operationalBuffer float64
	minRatio          float64
	useBalance        bool
	simMode           bool
	l                 logger.Logger

	// uninitialized
	seqNum       uint64
	reloadSeqNum bool
	baseAmount   float64
}

// MakeDexAgent is the factory method
func MakeDexAgent(
	api *horizon.Client,
	sourceSeed string,
	tradingSeed string,
	sourceAccount string,
	tradingAccount string,
	network build.Network,
	threadTracker *multithreading.ThreadTracker,
	operationalBuffer float64,
	minRatio float64,
	useBalance bool,
	staticAmount float64,
	simMode bool,
	l logger.Logger,
) *DexAgent {
	dexAgent := &DexAgent{
		API:               api,
		SourceSeed:        sourceSeed,
		TradingSeed:       tradingSeed,
		SourceAccount:     sourceAccount,
		TradingAccount:    tradingAccount,
		Network:           network,
		threadTracker:     threadTracker,
		operationalBuffer: operationalBuffer,
		minRatio:          minRatio,
		useBalance:        useBalance,
		simMode:           simMode,
		l:                 l,
	}

	if dexAgent.SourceAccount == "" {
		dexAgent.SourceAccount = dexAgent.TradingAccount
		dexAgent.SourceSeed = dexAgent.TradingSeed
		l.Info("No Source Account Set")
	}

	if !useBalance {
		dexAgent.baseAmount = staticAmount
	}

	dexAgent.reloadSeqNum = true

	return dexAgent
}

func (dA *DexAgent) incrementSeqNum() {
	if dA.reloadSeqNum {
		dA.l.Info("reloading sequence number")
		seqNum, err := dA.API.SequenceForAccount(dA.SourceAccount)
		if err != nil {
			dA.l.Infof("error getting seq num: %s\n", err)
			return
		}
		dA.seqNum = uint64(seqNum)
		dA.reloadSeqNum = false
	}
	dA.seqNum++
}

// assetBalance returns asset balance, asset trust limit, reserve balance, error
func (dA *DexAgent) assetBalance(asset horizon.Asset) (float64, float64, float64, error) {
	account, err := dA.API.LoadAccount(dA.TradingAccount)
	if err != nil {
		return -1, -1, -1, fmt.Errorf("error: unable to load account to fetch balance: %s", err)
	}

	for _, balance := range account.Balances {
		if utils.AssetsEqual(balance.Asset, asset) {
			b, e := strconv.ParseFloat(balance.Balance, 64)
			if e != nil {
				return -1, -1, -1, fmt.Errorf("error: cannot parse balance: %s", e)
			}
			if balance.Asset.Type == utils.Native {
				return b, maxLumenTrust, dA.minReserve(account.SubentryCount) + dA.operationalBuffer, e
			}
		}
	}
	return -1, -1, -1, errors.New("could not find a balance for the asset passed in")
}

// JustAssetBalance returns asset balance
func (dA *DexAgent) JustAssetBalance(asset horizon.Asset) (float64, error) {
	account, err := dA.API.LoadAccount(dA.TradingAccount)
	if err != nil {
		return -1, fmt.Errorf("error: unable to load account to fetch balance: %s", err)
	}

	for _, balance := range account.Balances {
		if utils.AssetsEqual(balance.Asset, asset) {
			b, e := strconv.ParseFloat(balance.Balance, 64)
			if e != nil {
				return -1, fmt.Errorf("error: cannot parse balance: %s", e)
			}
			if balance.Asset.Type == utils.Native {
				return b - (dA.minReserve(account.SubentryCount) + dA.operationalBuffer), e
			}
		}
	}
	return -1, errors.New("could not find a balance for the asset passed in")
}

// SubmitOps submits the passed in operations to the network asynchronously in a single transaction
func (dA *DexAgent) SubmitOps(ops []build.TransactionMutator, asyncCallback func(hash string, e error)) error {
	dA.incrementSeqNum()
	muts := []build.TransactionMutator{
		build.Sequence{Sequence: dA.seqNum},
		dA.Network,
		build.SourceAccount{AddressOrSeed: dA.SourceAccount},
	}
	muts = append(muts, ops...)
	tx, e := build.Transaction(muts...)
	if e != nil {
		return errors.Wrap(e, "SubmitOps error: ")
	}

	// convert to xdr string
	txeB64, e := dA.sign(tx)
	if e != nil {
		return e
	}
	dA.l.Infof("tx XDR: %s\n", txeB64)

	// submit
	if !dA.simMode {
		dA.l.Info("submitting tx XDR to network (async)")
		dA.threadTracker.TriggerGoroutine(func(inputs []interface{}) {
			dA.submit(txeB64, asyncCallback)
		}, nil)
	} else {
		dA.l.Info("not submitting tx XDR to network in simulation mode, calling asyncCallback with empty hash value")
		dA.invokeAsyncCallback(asyncCallback, "", nil)
	}
	return nil
}

func (dA *DexAgent) sign(tx *build.TransactionBuilder) (string, error) {
	var txe build.TransactionEnvelopeBuilder
	var e error

	if dA.SourceSeed != dA.TradingSeed {
		txe, e = tx.Sign(dA.SourceSeed, dA.TradingSeed)
	} else {
		txe, e = tx.Sign(dA.SourceSeed)
	}
	if e != nil {
		return "", e
	}

	return txe.Base64()
}

func (dA *DexAgent) submit(txeB64 string, asyncCallback func(hash string, e error)) {
	resp, err := dA.API.SubmitTransaction(txeB64)
	if err != nil {
		if herr, ok := errors.Cause(err).(*horizon.Error); ok {
			var rcs *horizon.TransactionResultCodes
			rcs, err = herr.ResultCodes()
			if err != nil {
				dA.l.Infof("(async) error: no result codes from horizon: %s\n", err)
				dA.invokeAsyncCallback(asyncCallback, "", err)
				return
			}
			if rcs.TransactionCode == "tx_bad_seq" {
				dA.l.Info("(async) error: tx_bad_seq, setting flag to reload seq number")
				dA.reloadSeqNum = true
			}
			dA.l.Infof("(async) error: result code details: tx code =", rcs.TransactionCode, ", opcodes =", rcs.OperationCodes)
		} else {
			dA.l.Infof("(async) error: tx failed for unknown reason, error message: %s\n", err)
		}
		dA.invokeAsyncCallback(asyncCallback, "", err)
		return
	}

	dA.l.Infof("(async) tx confirmation hash: %s\n", resp.Hash)
	dA.invokeAsyncCallback(asyncCallback, resp.Hash, nil)
}

func (dA *DexAgent) invokeAsyncCallback(asyncCallback func(hash string, e error), hash string, e error) {
	if asyncCallback == nil {
		return
	}

	dA.threadTracker.TriggerGoroutine(func(inputs []interface{}) {
		asyncCallback(hash, e)
	}, nil)
}

func (dA *DexAgent) minReserve(subentries int32) float64 {
	return float64(2+subentries) * baseReserve
}

// SendPaymentCycle executes a payment cycle
func (dA *DexAgent) SendPaymentCycle(path *PaymentPath, maxAmount float64) error {
	if dA.useBalance {
		e := dA.payWithBalance(path, maxAmount)
		if e != nil {
			return fmt.Errorf("error while preparing to send payment cycle %s", e)
		}
		return nil
	}

	e := dA.payWithAmount(path, maxAmount)
	if e != nil {
		return fmt.Errorf("error while preparing to send payment cycle %s", e)
	}
	return nil
}

func (dA *DexAgent) payWithBalance(path *PaymentPath, maxAmount float64) error {
	balance, e := dA.JustAssetBalance(path.HoldAsset)
	if e != nil {
		return fmt.Errorf("error getting account hold asset balance %s", e)
	}
	payAmount := balance
	if maxAmount < balance {
		payAmount = maxAmount
	}
	payOp, e := dA.MakePathPayment(path, payAmount, dA.minRatio)
	if e != nil {
		return fmt.Errorf("Error submitting path payment op: %s", e)
	}
	var opList []build.TransactionMutator
	//opList will be a list of one because the submission func wants a list (so it can do multi-op transactions)
	opList = append(opList, payOp)

	e = dA.SubmitOps(opList, nil)
	if e != nil {
		return fmt.Errorf("Error while submitting path payment op: %s", e)
	}

	return nil
}

func (dA *DexAgent) payWithAmount(path *PaymentPath, maxAmount float64) error {
	payAmount := dA.baseAmount
	if maxAmount < dA.baseAmount {
		payAmount = maxAmount
	}

	payOp, e := dA.MakePathPayment(path, payAmount, dA.minRatio)
	if e != nil {
		return fmt.Errorf("Error submitting path payment op: %s", e)
	}

	var opList []build.TransactionMutator
	//opList will be a list of one because the submission func wants a list (so it can do multi-op transactions)
	opList = append(opList, payOp)

	e = dA.SubmitOps(opList, nil)
	if e != nil {
		return fmt.Errorf("Error submitting path payment op: %s", e)
	}

	return nil
}

// MakePathPayment constructs and returns a path payment transaction for a given path object
func (dA *DexAgent) MakePathPayment(path *PaymentPath, amount float64, minRatio float64) (build.PaymentBuilder, error) {
	holdAsset, throughAssetA, throughAssetB := Path2Assets(path)
	receiveAmount := model.NumberFromFloat(amount, utils.SdexPrecision).AsString()
	maxPayAmount := model.NumberFromFloat(amount*(1/minRatio), utils.SdexPrecision).AsString()
	dA.l.Infof("Set receiveAmount to: %s", receiveAmount)
	dA.l.Infof("Set maxPayAmount to: %s", receiveAmount)

	pw := build.PayWith(utils.Asset2Asset(holdAsset), maxPayAmount)
	pw = pw.Through(utils.Asset2Asset(throughAssetA))
	pw = pw.Through(utils.Asset2Asset(throughAssetB))

	// payDest := build.Destination{
	// 	AddressOrSeed: dA.TradingAccount,
	// }

	if utils.Asset2Asset(holdAsset) == build.NativeAsset() {
		// payAmount := build.NativeAmount{
		// 	Amount: receiveAmount}

		payOp := build.Payment(
			build.Destination{AddressOrSeed: dA.TradingAccount},
			build.NativeAmount{Amount: receiveAmount},
			pw,
		)

		// payOp.Mutate(
		// 	build.Destination{AddressOrSeed: dA.TradingAccount},
		// 	path.HoldAsset,
		// 	build.NativeAmount{Amount: receiveAmount},
		// 	//pw,
		// )

		// tx, e := build.Transaction(
		// 	build.Sequence{Sequence: dA.seqNum},
		// 	dA.Network,
		// 	build.SourceAccount{AddressOrSeed: dA.SourceAccount},
		// 	payOp,
		// )
		// if e != nil {
		// 	return nil, fmt.Errorf("Tranaction build failed: %s", e)
		// }

		return payOp, nil
	}

	// payAmount := build.CreditAmount{
	// 	Code:   holdAsset.Code,
	// 	Issuer: holdAsset.Issuer,
	// 	Amount: receiveAmount,
	// }

	payOp := build.Payment(
		build.Destination{AddressOrSeed: dA.TradingAccount},
		build.CreditAmount{
			Code:   holdAsset.Code,
			Issuer: holdAsset.Issuer,
			Amount: receiveAmount,
		},
		pw,
	)

	// tx, e := build.Transaction(
	// 	build.Sequence{Sequence: dA.seqNum},
	// 	dA.Network,
	// 	build.SourceAccount{AddressOrSeed: dA.SourceAccount},
	// 	payOp,
	// )
	// if e != nil {
	// 	return nil, fmt.Errorf("Tranaction build failed: %s", e)
	// }

	return payOp, nil

	// payOp.Mutate(
	// 	build.Destination{
	// 		AddressOrSeed: dA.TradingAccount,
	// 	},
	// 	path.HoldAsset,
	// 	build.CreditAmount{
	// 		Code:   holdAsset.Code,
	// 		Issuer: holdAsset.Issuer,
	// 		Amount: receiveAmount,
	// 	},
	//pw,
	// )
}

// SumbitPayments submits a fully built payment transaction to the network asynchronously in a single transaction
func (dA *DexAgent) SumbitPayments(tx *build.TransactionBuilder, asyncCallback func(hash string, e error)) error {
	//dA.incrementSeqNum()
	// muts := []build.TransactionMutator{
	// 	build.Sequence{Sequence: dA.seqNum},
	// 	dA.Network,
	// 	build.SourceAccount{AddressOrSeed: dA.SourceAccount},
	// }
	// muts = append(muts, ops...)
	// tx, e := build.Transaction(muts...)
	// if e != nil {
	// 	return errors.Wrap(e, "SubmitOps error: ")
	// }

	// convert to xdr string
	txeB64, e := dA.sign(tx)
	if e != nil {
		return e
	}
	dA.l.Infof("tx XDR: %s\n", txeB64)

	// submit
	if !dA.simMode {
		dA.l.Info("submitting tx XDR to network (async)")
		dA.threadTracker.TriggerGoroutine(func(inputs []interface{}) {
			dA.submit(txeB64, asyncCallback)
		}, nil)
	} else {
		dA.l.Info("not submitting tx XDR to network in simulation mode, calling asyncCallback with empty hash value")
		dA.invokeAsyncCallback(asyncCallback, "", nil)
	}
	return nil
}
