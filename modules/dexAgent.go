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
	rateLimiter       func()
	submitDone        chan<- bool
	operationalBuffer float64
	minRatio          *model.Number
	simMode           bool
	l                 logger.Logger

	// uninitialized
	seqNum       uint64
	reloadSeqNum bool
}

// TransData contains the data needed to contruct a path payment
type TransData struct {
	Path   *PaymentPath
	Amount *model.Number
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
	rateLimiter func(),
	submitDone chan<- bool,
	operationalBuffer float64,
	minRatio float64,
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
		rateLimiter:       rateLimiter,
		submitDone:        submitDone,
		operationalBuffer: operationalBuffer,
		minRatio:          model.NumberFromFloat(minRatio, utils.SdexPrecision),
		simMode:           simMode,
		l:                 l,
	}

	if dexAgent.SourceAccount == "" {
		dexAgent.SourceAccount = dexAgent.TradingAccount
		dexAgent.SourceSeed = dexAgent.TradingSeed
		l.Info("No Source Account Set")
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
			return b, e
		}
	}
	return -1, errors.New("could not find a balance for the asset passed in")
}

// SubmitOps submits the passed in operations to the network asynchronously in a single transaction
func (dA *DexAgent) SubmitOps(ops []build.TransactionMutator, asyncCallback func(hash string, e error)) error {
	dA.reloadSeqNum = true
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

	// dA.l.Infof("pre-XDR raw was: %s", tx)
	// convert to xdr string
	txeB64, e := dA.sign(tx)
	if e != nil {
		return e
	}
	// dA.l.Infof("tx XDR: %s\n", txeB64)

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
func (dA *DexAgent) SendPaymentCycle(path *PaymentPath, maxAmount *model.Number) error {

	payOp, e := dA.makePathPayment(path, maxAmount)
	if e != nil {
		return fmt.Errorf("error submitting path payment op: %s", e)
	}

	var opList []build.TransactionMutator
	//opList will be a list of one because the submission func wants a list (so it can do multi-op transactions)
	opList = append(opList, payOp)

	e = dA.SubmitOps(opList, nil)
	if e != nil {
		return fmt.Errorf("error submitting path payment op: %s", e)
	}

	return nil
}

// makePathPayment constructs and returns a path payment transaction for a given path object
func (dA *DexAgent) makePathPayment(path *PaymentPath, maxAmount *model.Number) (*build.PaymentBuilder, error) {
	cycleAmount := maxAmount

	if path.UseBalance {
		balance, e := dA.JustAssetBalance(path.HoldAsset)
		if e != nil {
			return nil, fmt.Errorf("error getting account hold asset balance %s", e)
		}
		if balance < maxAmount.AsFloat() {
			cycleAmount = model.NumberFromFloat(balance, utils.SdexPrecision)
		}
	}

	if !path.UseBalance {
		if path.StaticAmount.AsFloat() < maxAmount.AsFloat() {
			cycleAmount = path.StaticAmount
		}
	}

	// if cycleAmount is still maxAmount, reduce it to allow for stellar core price rounding
	// if the price gets rounded against us the payment will slip to the next offer and end up over source max
	// core should round by no more than 1%
	// going down to 0.75 to test the op-over thing
	if cycleAmount.AsFloat() >= maxAmount.AsFloat() {
		cycleAmount = maxAmount.Scale(0.75)
	}

	holdAsset, throughAssetA, throughAssetB := Path2Assets(path)

	receiveAmount := cycleAmount.AsString()
	// currently set these to equal to allow max successes to diagnose over-max problem
	maxPayAmount := cycleAmount.AsString()

	dA.l.Infof("receiveAmount string set to: %s", receiveAmount)
	dA.l.Infof("maxPayAmount string set to: %s", maxPayAmount)

	//pre-converting before building in case doing the conversion inside the build.Paywith or pw.Through is screwing stuff up
	convertHoldAsset := utils.Asset2Asset(holdAsset)
	convertAssetA := utils.Asset2Asset(throughAssetA)
	convertAssetB := utils.Asset2Asset(throughAssetB)

	pw := build.PayWith(convertHoldAsset, maxPayAmount)
	pw = pw.Through(convertAssetA)
	pw = pw.Through(convertAssetB)

	if convertHoldAsset == build.NativeAsset() {

		payOp := build.Payment(
			build.Destination{AddressOrSeed: dA.TradingAccount},
			build.NativeAmount{Amount: receiveAmount},
			pw,
		)

		return &payOp, nil
	}

	payOp := build.Payment(
		build.Destination{AddressOrSeed: dA.TradingAccount},
		build.CreditAmount{
			Code:   convertHoldAsset.Code,
			Issuer: convertHoldAsset.Issuer,
			Amount: receiveAmount,
		},
		pw,
	)

	return &payOp, nil

}

// SendByFoundPath executes a payment cycle for the find-path protocol
// deprecated until pathRequestor is rebuilt
// func (dA *DexAgent) SendByFoundPath(path *PathRecord, holdAsset *horizon.Asset, maxAmount *model.Number, hold chan bool, done chan bool) error {

// 	payOp, e := dA.makePathPaymentredux(path, holdAsset, maxAmount)
// 	if e != nil {
// 		return fmt.Errorf("error trying to send via found path")
// 	}

// 	var opList []build.TransactionMutator
// 	//opList will be a list of one because the submission func wants a list (so it can do multi-op transactions)
// 	opList = append(opList, payOp)

// 	e = dA.SubmitOps(opList, nil)
// 	if e != nil {
// 		return fmt.Errorf("error submitting path payment op: %s", e)
// 	}

// 	return nil
// }

// makePathPaymentredux contructs a path payment from a find-path PathRecord
// deprecated until pathRequestor is rebuilt
// func (dA *DexAgent) makePathPaymentredux(payPath *PathRecord, holdAsset *horizon.Asset, amount *model.Number) (*build.PaymentBuilder, error) {

// 	// if not using balance the amount will have already been adjusted to the static amount
// 	if dA.useBalance == true {
// 		balance, e := dA.JustAssetBalance(*holdAsset)
// 		numBalanace := model.NumberFromFloat(balance, utils.SdexPrecision)
// 		if e != nil {
// 			return nil, fmt.Errorf("error creating path payment op: %s", e)
// 		}

// 		if numBalanace.AsFloat() < amount.AsFloat() {
// 			amount = numBalanace
// 		}

// 	}

// 	receiveAmount := amount.AsString()

// 	// use the literal horizon output for cost, yeesh
// 	maxPayAmount := payPath.SourceAmount
// 	numMax, e := model.NumberFromString(maxPayAmount, utils.SdexPrecision)
// 	if e != nil {
// 		return nil, fmt.Errorf("error converting source amount string to model.number: %s", e)
// 	}

// 	// with a stop to make sure it's not over somehow
// 	if numMax.AsFloat() > amount.AsFloat() {
// 		maxPayAmount = amount.AsString()
// 	}

// 	dA.l.Infof("receiveAmount string set to: %s", receiveAmount)
// 	dA.l.Infof("maxPayAmount string set to: %s", maxPayAmount)

// 	convertHoldAsset := utils.Asset2Asset(*holdAsset)

// 	pw := build.PayWith(convertHoldAsset, maxPayAmount)

// 	for i := 0; i < len(payPath.Path); i++ {
// 		dA.l.Infof("Adding to payment path: %s|%s", payPath.Path[i].AssetCode, payPath.Path[i].AssetIssuer)
// 		convertAsset := PathAsset2BuildAsset(payPath.Path[i])
// 		pw = pw.Through(convertAsset)
// 	}

// 	// if not calculating the full path you must add the asset set as the "destination" as a path asset
// 	lastPathAsset := PathAsset{
// 		AssetType:   payPath.DestinationAssetType,
// 		AssetCode:   payPath.DestinationAssetCode,
// 		AssetIssuer: payPath.DestinationAssetIssuer,
// 	}

// 	lastConvertAsset := PathAsset2BuildAsset(lastPathAsset)

// 	pw = pw.Through(lastConvertAsset)
// 	dA.l.Infof("adding to payment path: %s|%s", lastPathAsset.AssetCode, lastPathAsset.AssetIssuer)

// 	dA.l.Infof("raw build.PayWith set to: %s", pw)

// 	if convertHoldAsset == build.NativeAsset() {

// 		payOp := build.Payment(
// 			build.Destination{AddressOrSeed: dA.TradingAccount},
// 			build.NativeAmount{Amount: receiveAmount},
// 			pw,
// 		)
// 		return &payOp, nil
// 	}

// 	payOp := build.Payment(
// 		build.Destination{AddressOrSeed: dA.TradingAccount},
// 		build.CreditAmount{
// 			Code:   convertHoldAsset.Code,
// 			Issuer: convertHoldAsset.Issuer,
// 			Amount: receiveAmount,
// 		},
// 		pw,
// 	)

// 	return &payOp, nil
// }
