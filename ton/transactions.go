package ton

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/xssnick/tonutils-go/tl"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

func init() {
	tl.Register(GetOneTransaction{}, "liteServer.getOneTransaction id:tonNode.blockIdExt account:liteServer.accountId lt:long = liteServer.TransactionInfo")
	tl.Register(GetTransactions{}, "liteServer.getTransactions count:# account:liteServer.accountId lt:long hash:int256 = liteServer.TransactionList")
}

type GetOneTransaction struct {
	ID    *tlb.BlockInfo `tl:"struct"`
	AccID *AccountID     `tl:"struct"`
	LT    int64          `tl:"long"`
}

type GetTransactions struct {
	Limit  int32      `tl:"int"`
	AccID  *AccountID `tl:"struct"`
	LT     int64      `tl:"long"`
	TxHash []byte     `tl:"int256"`
}

// ListTransactions - returns list of transactions before (including) passed lt and hash, the oldest one is first in result slice
func (c *APIClient) ListTransactions(ctx context.Context, addr *address.Address, limit uint32, lt uint64, txHash []byte) ([]*tlb.Transaction, error) {
	resp, err := c.client.DoRequest(ctx, GetTransactions{
		Limit: int32(limit),
		AccID: &AccountID{
			WorkChain: addr.Workchain(),
			ID:        addr.Data(),
		},
		LT:     int64(lt),
		TxHash: txHash,
	})
	if err != nil {
		return nil, err
	}

	switch resp.TypeID {
	case _TransactionsList:
		if len(resp.Data) <= 4 {
			return nil, errors.New("too short response")
		}

		vecLn := binary.LittleEndian.Uint32(resp.Data)
		resp.Data = resp.Data[4:]

		for i := 0; i < int(vecLn); i++ {
			var block tlb.BlockInfo

			resp.Data, err = block.Load(resp.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to load block from vector: %w", err)
			}
		}

		var txData []byte
		txData, resp.Data, err = tl.FromBytes(resp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to load transaction bytes: %w", err)
		}

		txList, err := cell.FromBOCMultiRoot(txData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cell from transaction bytes: %w", err)
		}

		res := make([]*tlb.Transaction, 0, len(txList))
		for _, txCell := range txList {
			loader := txCell.BeginParse()

			var tx tlb.Transaction
			err = tlb.LoadFromCell(&tx, loader)
			if err != nil {
				return nil, fmt.Errorf("failed to load transaction from cell: %w", err)
			}
			tx.Hash = txCell.Hash()

			res = append(res, &tx)
		}

		return res, nil
	case _LSError:
		var lsErr LSError
		resp.Data, err = lsErr.Load(resp.Data)
		if err != nil {
			return nil, err
		}

		if lsErr.Code == 0 {
			return nil, ErrMessageNotAccepted
		}

		return nil, lsErr
	}

	return nil, errors.New("unknown response type")
}

func (c *APIClient) GetTransaction(ctx context.Context, block *tlb.BlockInfo, addr *address.Address, lt uint64) (*tlb.Transaction, error) {
	resp, err := c.client.DoRequest(ctx, GetOneTransaction{
		ID: block,
		AccID: &AccountID{
			WorkChain: addr.Workchain(),
			ID:        addr.Data(),
		},
		LT: int64(lt),
	})
	if err != nil {
		return nil, err
	}

	switch resp.TypeID {
	case _TransactionInfo:
		b := new(tlb.BlockInfo)
		resp.Data, err = b.Load(resp.Data)
		if err != nil {
			return nil, err
		}

		var proof []byte
		proof, resp.Data, err = tl.FromBytes(resp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to load proof bytes: %w", err)
		}
		_ = proof

		var txData []byte
		txData, resp.Data, err = tl.FromBytes(resp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to load transaction bytes: %w", err)
		}

		txCell, err := cell.FromBOC(txData)
		if err != nil {
			return nil, fmt.Errorf("failed to parrse cell from transaction bytes: %w", err)
		}

		var tx tlb.Transaction
		err = tlb.LoadFromCell(&tx, txCell.BeginParse())
		if err != nil {
			return nil, fmt.Errorf("failed to load transaction from cell: %w", err)
		}
		tx.Hash = txCell.Hash()

		return &tx, nil
	case _LSError:
		var lsErr LSError
		resp.Data, err = lsErr.Load(resp.Data)
		if err != nil {
			return nil, err
		}

		if lsErr.Code == 0 {
			return nil, ErrMessageNotAccepted
		}

		return nil, lsErr
	}

	return nil, errors.New("unknown response type")
}
