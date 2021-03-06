// Copyright (c) 2017 The Decred developers
// Copyright (c) 2019 Romano (Viacoin developer)
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package atomic

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/romanornr/AtomicOTCswap/bcoins"
	"github.com/viacoin/viad/chaincfg"
	btcutil "github.com/viacoin/viautil"
	"time"
)

type initiateCmd struct {
	counterParty2Addr *btcutil.AddressPubKeyHash
	amount            btcutil.Amount
}

type InitiatedContract struct {
	Coin                   string  `json:"coin"`
	Unit                   string  `json:"unit"`
	ContractAmount         float64 `json:"contract_amount"`
	ContractFee            float64 `json:"contract_fee"`
	ContractRefundFee      float64 `json:"contract_refund_fee"`
	CounterPartyAddress    string  `json:"counter_party_address"`
	ContractAddress        string  `json:"contract_address"`
	ContractHex            string  `json:"contract_hex"`
	ContractTransactionID  string  `json:"contract_transaction_id"`
	ContractTransactionHex string  `json:"contract_transaction_hex"`
	RefundTransactionID    string  `json:"refund_transaction_id"`
	RefundTransaction      string  `json:"refund_transaction"`
	Secret                 string  `json:"secret"`
	SecretHash             string  `json:"secret_hash"`
}

func Initiate(coinTicker string, participantAddr string, amount float64, WIFstring string) (contract InitiatedContract, err error) {

	coin, err := bcoins.SelectCoin(coinTicker)
	if err != nil {
		return contract, err
	}

	chaincfg.Register(coin.Network.ChainCgfMainNetParams())

	wif, err := btcutil.DecodeWIF(WIFstring)
	if err != nil {
		return contract, fmt.Errorf("error decoding private key in wif format: %s", err)
	}

	counterParty2Addr, err := btcutil.DecodeAddress(participantAddr, coin.Network.ChainCgfMainNetParams())
	if err != nil {
		return contract, fmt.Errorf("failed to decode the address from the participant: %s", err)
	}

	counterParty2AddrP2KH, ok := counterParty2Addr.(*btcutil.AddressPubKeyHash)
	if !ok {
		return contract, errors.New("participant address is not P2KH")
	}

	amount2, err := btcutil.NewAmount(amount)
	if err != nil {
		return contract, err
	}

	cmd := &initiateCmd{counterParty2Addr: counterParty2AddrP2KH, amount: amount2}
	return cmd.runCommand(wif, &coin, amount)
}

func (cmd *initiateCmd) runCommand(wif *btcutil.WIF, coin *bcoins.Coin, amount float64) (InitiatedContract, error) {
	var secret [secretSize]byte
	_, err := rand.Read(secret[:])
	if err != nil {
		return InitiatedContract{}, err
	}

	secretHash := sha256Hash(secret[:])
	locktime := time.Now().Add(24 * time.Hour).Unix() // NEED TO CHANGE

	build, err := buildContract(&contractArgs{
		coin:       coin,
		them:       cmd.counterParty2Addr,
		amount:     cmd.amount,
		locktime:   locktime,
		secretHash: secretHash,
	}, wif)

	if err != nil {
		return InitiatedContract{}, err
	}

	refundTxHash := build.refundTx.TxHash()

	var contractBuf bytes.Buffer
	contractBuf.Grow(build.contractTx.SerializeSize())
	build.contractTx.Serialize(&contractBuf)

	var refundBuf bytes.Buffer
	refundBuf.Grow(build.refundTx.SerializeSize())
	build.refundTx.Serialize(&refundBuf)

	contract := InitiatedContract{

		Secret:     fmt.Sprintf("%x", secret),
		SecretHash: fmt.Sprintf("%x", secretHash),

		Coin: coin.Name,
		Unit: coin.Unit,

		ContractAmount:    amount,
		ContractFee:       build.contractFee.ToBTC(),
		ContractRefundFee: build.refundFee.ToBTC(),

		CounterPartyAddress: fmt.Sprintf("%s", cmd.counterParty2Addr.String()),
		ContractAddress:     fmt.Sprintf("%v", build.contractP2SH),
		ContractHex:         fmt.Sprintf("%x", build.contract),

		ContractTransactionID:  fmt.Sprintf("%x", build.contractTxHash),
		ContractTransactionHex: fmt.Sprintf("%x", contractBuf.Bytes()),

		RefundTransactionID: fmt.Sprintf("%v", &refundTxHash),
		RefundTransaction:   fmt.Sprintf("%x", refundBuf.Bytes()),
	}
	return contract, nil
}
