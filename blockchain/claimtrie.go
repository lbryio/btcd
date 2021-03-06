package blockchain

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"github.com/btcsuite/btcd/claimtrie"
	"github.com/btcsuite/btcd/claimtrie/node"
)

// Hack: print which block mismatches happened, but keep recording.
var mismatchedPrinted bool

func (b *BlockChain) ParseClaimScripts(block *btcutil.Block, node *blockNode, view *UtxoViewpoint, failOnHashMiss bool) error {
	ht := block.Height()

	for _, tx := range block.Transactions() {
		h := handler{ht, tx, view, map[string][]byte{}}
		if err := h.handleTxIns(b.claimTrie); err != nil {
			return err
		}
		if err := h.handleTxOuts(b.claimTrie); err != nil {
			return err
		}
	}

	// Hack: let the claimtrie know the expected Hash.
	b.claimTrie.ReportHash(ht, node.claimTrie)

	err := b.claimTrie.AppendBlock()
	if err != nil {
		return err
	}
	hash := b.claimTrie.MerkleHash()

	if node.claimTrie != *hash {
		if failOnHashMiss {
			return fmt.Errorf("height: %d, ct.MerkleHash: %s != node.ClaimTrie: %s", ht, *hash, node.claimTrie)
		}
		if !mismatchedPrinted {
			fmt.Printf("\n\nHeight: %d, ct.MerkleHash: %s != node.ClaimTrie: %s, Error: %s\n", ht, *hash, node.claimTrie, err)
			mismatchedPrinted = true
		}
	}
	return nil
}

type handler struct {
	ht    int32
	tx    *btcutil.Tx
	view  *UtxoViewpoint
	spent map[string][]byte
}

func (h *handler) handleTxIns(ct *claimtrie.ClaimTrie) error {
	if IsCoinBase(h.tx) {
		return nil
	}
	for _, txIn := range h.tx.MsgTx().TxIn {
		op := txIn.PreviousOutPoint
		e := h.view.LookupEntry(op)
		if e == nil {
			return fmt.Errorf("missing input in view for %s", op.String())
		}
		cs, err := txscript.DecodeClaimScript(e.pkScript)
		if err == txscript.ErrNotClaimScript {
			continue
		}
		if err != nil {
			return err
		}

		var id node.ClaimID
		name := cs.Name() // name of the previous one (that we're now spending)

		switch cs.Opcode() {
		case txscript.OP_CLAIMNAME: // OP code from previous transaction
			id = node.NewClaimID(op) // claimID of the previous item now being spent
			h.spent[id.String()] = node.NormalizeIfNecessary(name, ct.Height())
			err = ct.SpendClaim(name, op, id)
		case txscript.OP_UPDATECLAIM:
			copy(id[:], cs.ClaimID())
			h.spent[id.String()] = node.NormalizeIfNecessary(name, ct.Height())
			err = ct.SpendClaim(name, op, id)
		case txscript.OP_SUPPORTCLAIM:
			copy(id[:], cs.ClaimID())
			err = ct.SpendSupport(name, op, id)
		}
		if err != nil {
			return errors.Wrapf(err, "handleTxIns")
		}
	}
	return nil
}

func (h *handler) handleTxOuts(ct *claimtrie.ClaimTrie) error {
	for i, txOut := range h.tx.MsgTx().TxOut {
		op := wire.NewOutPoint(h.tx.Hash(), uint32(i))
		cs, err := txscript.DecodeClaimScript(txOut.PkScript)
		if err == txscript.ErrNotClaimScript {
			continue
		}
		if err != nil {
			return err
		}

		var id node.ClaimID
		name := cs.Name()
		amt := txOut.Value
		value := cs.Value()

		switch cs.Opcode() {
		case txscript.OP_CLAIMNAME:
			id = node.NewClaimID(*op)
			err = ct.AddClaim(name, *op, id, amt, value)
		case txscript.OP_SUPPORTCLAIM:
			copy(id[:], cs.ClaimID())
			err = ct.AddSupport(name, value, *op, amt, id)
		case txscript.OP_UPDATECLAIM:
			// old code wouldn't run the update if name or claimID didn't match existing data
			// that was a safety feature, but it should have rejected the transaction instead
			// TODO: reject transactions with invalid update commands
			copy(id[:], cs.ClaimID())
			normName := node.NormalizeIfNecessary(name, ct.Height())
			if !bytes.Equal(h.spent[id.String()], normName) {
				fmt.Printf("Invalid update operation: name or ID mismatch for %s, %s\n", normName, id.String())
				continue
			}

			delete(h.spent, id.String())
			err = ct.UpdateClaim(name, *op, amt, id, value)
		}
		if err != nil {
			return errors.Wrapf(err, "handleTxOuts")
		}
	}
	return nil
}
