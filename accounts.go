package dcrlibwallet

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrwallet/errors"
	"github.com/decred/dcrwallet/wallet"
	"github.com/raedahgroup/dcrlibwallet/addresshelper"
	"strings"
)

func (lw *LibWallet) GetAccounts(requiredConfirmations int32) (string, error) {
	accountsResponse, err := lw.GetAccountsRaw(requiredConfirmations)
	if err != nil {
		return "", nil
	}

	result, _ := json.Marshal(accountsResponse)
	return string(result), nil
}

func (lw *LibWallet) GetAccountsRaw(requiredConfirmations int32) (*Accounts, error) {
	resp, err := lw.wallet.Accounts()
	if err != nil {
		return nil, err
	}
	accounts := make([]*Account, len(resp.Accounts))
	for i, account := range resp.Accounts {
		balance, err := lw.GetAccountBalance(int32(account.AccountNumber), requiredConfirmations)
		if err != nil {
			return nil, err
		}

		accounts[i] = &Account{
			Number:           int32(account.AccountNumber),
			Name:             account.AccountName,
			TotalBalance:     int64(account.TotalBalance),
			Balance:          balance,
			ExternalKeyCount: int32(account.LastUsedExternalIndex + 20),
			InternalKeyCount: int32(account.LastUsedInternalIndex + 20),
			ImportedKeyCount: int32(account.ImportedKeyCount),
		}
	}

	return &Accounts{
		Count:              len(resp.Accounts),
		CurrentBlockHash:   resp.CurrentBlockHash[:],
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Acc:                accounts,
		ErrorOccurred:      false,
	}, nil
}

func (lw *LibWallet) GetAccountBalance(accountNumber int32, requiredConfirmations int32) (*Balance, error) {
	balance, err := lw.wallet.CalculateAccountBalance(uint32(accountNumber), requiredConfirmations)
	if err != nil {
		return nil, err
	}

	return &Balance{
		Total:                   int64(balance.Total),
		Spendable:               int64(balance.Spendable),
		ImmatureReward:          int64(balance.ImmatureCoinbaseRewards),
		ImmatureStakeGeneration: int64(balance.ImmatureStakeGeneration),
		LockedByTickets:         int64(balance.LockedByTickets),
		VotingAuthority:         int64(balance.VotingAuthority),
		UnConfirmed:             int64(balance.Unconfirmed),
	}, nil
}

func (lw *LibWallet) SpendableForAccount(account int32, requiredConfirmations int32) (int64, error) {
	bals, err := lw.wallet.CalculateAccountBalance(uint32(account), requiredConfirmations)
	if err != nil {
		log.Error(err)
		return 0, translateError(err)
	}
	return int64(bals.Spendable), nil
}

func (lw *LibWallet) AllUnspentOutputs(account int32, requiredConfirmations int32) ([]*UnspentOutput, error) {
	policy := wallet.OutputSelectionPolicy{
		Account:               uint32(account),
		RequiredConfirmations: requiredConfirmations,
	}

	// fetch all utxos in account to extract details for the utxos selected by user
	// use targetAmount = 0 to fetch ALL utxos in account
	inputDetail, err := lw.wallet.SelectInputs(dcrutil.Amount(0), policy)

	if err != nil {
		return nil, err
	}

	unspentOutputs := make([]*UnspentOutput, len(inputDetail.Inputs))

	for i, input := range inputDetail.Inputs {
		outputInfo, err := lw.wallet.OutputInfo(&input.PreviousOutPoint)
		if err != nil {
			return nil, err
		}

		// unique key to identify utxo
		outputKey := fmt.Sprintf("%s:%d", input.PreviousOutPoint.Hash, input.PreviousOutPoint.Index)

		addresses, err := addresshelper.PkScriptAddresses(lw.activeNet.Params, inputDetail.Scripts[i])
		if err != nil {
			return nil, fmt.Errorf("error reading address details for unspent output: %v", err)
		}

		previousTx, err := lw.GetTransactionRaw(input.PreviousOutPoint.Hash[:])
		if err != nil {
			return nil, fmt.Errorf("error reading tx details for unspent output: %v", err)
		}

		var confirmations int32 = 0
		if previousTx.BlockHeight != -1 {
			confirmations = lw.GetBestBlock() - previousTx.BlockHeight + 1
		}

		unspentOutputs[i] = &UnspentOutput{
			TransactionHash: input.PreviousOutPoint.Hash[:],
			OutputIndex:     input.PreviousOutPoint.Index,
			OutputKey:       outputKey,
			Tree:            int32(input.PreviousOutPoint.Tree),
			Amount:          int64(outputInfo.Amount),
			PkScript:        inputDetail.Scripts[i],
			ReceiveTime:     outputInfo.Received.Unix(),
			FromCoinbase:    outputInfo.FromCoinbase,
			Address:         strings.Join(addresses, ", "),
			Confirmations:   confirmations,
		}
	}

	return unspentOutputs, nil
}

func (lw *LibWallet) NextAccount(accountName string, privPass []byte) error {
	_, err := lw.NextAccountRaw(accountName, privPass)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (lw *LibWallet) NextAccountRaw(accountName string, privPass []byte) (uint32, error) {
	lock := make(chan time.Time, 1)
	defer func() {
		for i := range privPass {
			privPass[i] = 0
		}
		lock <- time.Time{} // send matters, not the value
	}()
	err := lw.wallet.Unlock(privPass, lock)
	if err != nil {
		log.Error(err)
		return 0, errors.New(ErrInvalidPassphrase)
	}

	return lw.wallet.NextAccount(accountName)
}

func (lw *LibWallet) RenameAccount(accountNumber int32, newName string) error {
	err := lw.wallet.RenameAccount(uint32(accountNumber), newName)
	return err
}

func (lw *LibWallet) AccountName(accountNumber int32) string {
	name, err := lw.AccountNameRaw(uint32(accountNumber))
	if err != nil {
		log.Error(err)
		return "Account not found"
	}
	return name
}

func (lw *LibWallet) AccountNameRaw(accountNumber uint32) (string, error) {
	return lw.wallet.AccountName(accountNumber)
}

func (lw *LibWallet) AccountNumber(accountName string) (uint32, error) {
	return lw.wallet.AccountNumber(accountName)
}
