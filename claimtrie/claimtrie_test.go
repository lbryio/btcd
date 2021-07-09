package claimtrie

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/claimtrie/merkletrie"
	"github.com/btcsuite/btcd/claimtrie/node"
	"github.com/btcsuite/btcd/claimtrie/param"
	"github.com/btcsuite/btcd/wire"

	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) {
	param.SetNetwork(wire.TestNet, "")
	param.ClaimtrieDataFolder = t.TempDir()
}

func b(s string) []byte {
	return []byte(s)
}

func buildTx(hash chainhash.Hash) *wire.MsgTx {
	tx := wire.NewMsgTx(1)
	txIn := wire.NewTxIn(wire.NewOutPoint(&hash, 0), nil, nil)
	tx.AddTxIn(txIn)
	tx.AddTxOut(wire.NewTxOut(0, nil))
	return tx
}

func TestFixedHashes(t *testing.T) {

	r := require.New(t)

	setup(t)
	ct, err := New(true)
	r.NoError(err)
	defer func() {
		err = ct.Close()
		r.NoError(err)
	}()

	r.Equal(merkletrie.EmptyTrieHash[:], ct.MerkleHash()[:])

	tx1 := buildTx(*merkletrie.EmptyTrieHash)
	tx2 := buildTx(tx1.TxHash())
	tx3 := buildTx(tx2.TxHash())
	tx4 := buildTx(tx3.TxHash())

	err = ct.AddClaim(b("test"), tx1.TxIn[0].PreviousOutPoint, node.NewClaimID(tx1.TxIn[0].PreviousOutPoint), 50, nil)
	r.NoError(err)

	err = ct.AddClaim(b("test2"), tx2.TxIn[0].PreviousOutPoint, node.NewClaimID(tx2.TxIn[0].PreviousOutPoint), 50, nil)
	r.NoError(err)

	err = ct.AddClaim(b("test"), tx3.TxIn[0].PreviousOutPoint, node.NewClaimID(tx3.TxIn[0].PreviousOutPoint), 50, nil)
	r.NoError(err)

	err = ct.AddClaim(b("tes"), tx4.TxIn[0].PreviousOutPoint, node.NewClaimID(tx4.TxIn[0].PreviousOutPoint), 50, nil)
	r.NoError(err)

	err = ct.AppendBlock()
	r.NoError(err)

	expected, err := chainhash.NewHashFromStr("938fb93364bf8184e0b649c799ae27274e8db5221f1723c99fb2acd3386cfb00")
	r.NoError(err)
	r.Equal(expected[:], ct.MerkleHash()[:])
}

func TestNormalizationFork(t *testing.T) {

	r := require.New(t)

	setup(t)
	param.NormalizedNameForkHeight = 2
	ct, err := New(true)
	r.NoError(err)
	r.NotNil(ct)
	defer func() {
		err = ct.Close()
		r.NoError(err)
	}()

	hash := chainhash.HashH([]byte{1, 2, 3})

	o1 := wire.OutPoint{Hash: hash, Index: 1}
	err = ct.AddClaim([]byte("AÑEJO"), o1, node.NewClaimID(o1), 10, nil)
	r.NoError(err)

	o2 := wire.OutPoint{Hash: hash, Index: 2}
	err = ct.AddClaim([]byte("AÑejo"), o2, node.NewClaimID(o2), 5, nil)
	r.NoError(err)

	o3 := wire.OutPoint{Hash: hash, Index: 3}
	err = ct.AddClaim([]byte("あてはまる"), o3, node.NewClaimID(o3), 5, nil)
	r.NoError(err)

	o4 := wire.OutPoint{Hash: hash, Index: 4}
	err = ct.AddClaim([]byte("Aḿlie"), o4, node.NewClaimID(o4), 5, nil)
	r.NoError(err)

	o5 := wire.OutPoint{Hash: hash, Index: 5}
	err = ct.AddClaim([]byte("TEST"), o5, node.NewClaimID(o5), 5, nil)
	r.NoError(err)

	o6 := wire.OutPoint{Hash: hash, Index: 6}
	err = ct.AddClaim([]byte("test"), o6, node.NewClaimID(o6), 7, nil)
	r.NoError(err)

	err = ct.AppendBlock()
	r.NoError(err)
	r.NotEqual(merkletrie.EmptyTrieHash[:], ct.MerkleHash()[:])

	n, err := ct.nodeManager.Node([]byte("AÑEJO"))
	r.NoError(err)
	r.NotNil(n.BestClaim)
	r.Equal(int32(1), n.TakenOverAt)

	o7 := wire.OutPoint{Hash: hash, Index: 7}
	err = ct.AddClaim([]byte("aÑEJO"), o7, node.NewClaimID(o7), 8, nil)
	r.NoError(err)

	err = ct.AppendBlock()
	r.NoError(err)
	r.NotEqual(merkletrie.EmptyTrieHash[:], ct.MerkleHash()[:])

	n, err = ct.nodeManager.Node([]byte("añejo"))
	r.NoError(err)
	r.Equal(3, len(n.Claims))
	r.Equal(uint32(1), n.BestClaim.OutPoint.Index)
	r.Equal(int32(2), n.TakenOverAt)
}

func TestActivationsOnNormalizationFork(t *testing.T) {

	r := require.New(t)

	setup(t)
	param.NormalizedNameForkHeight = 4
	ct, err := New(true)
	r.NoError(err)
	r.NotNil(ct)
	defer func() {
		err = ct.Close()
		r.NoError(err)
	}()

	hash := chainhash.HashH([]byte{1, 2, 3})

	o7 := wire.OutPoint{Hash: hash, Index: 7}
	err = ct.AddClaim([]byte("A"), o7, node.NewClaimID(o7), 1, nil)
	r.NoError(err)
	err = ct.AppendBlock()
	r.NoError(err)
	err = ct.AppendBlock()
	r.NoError(err)
	err = ct.AppendBlock()
	r.NoError(err)
	verifyBestIndex(t, ct, "A", 7, 1)

	o8 := wire.OutPoint{Hash: hash, Index: 8}
	err = ct.AddClaim([]byte("A"), o8, node.NewClaimID(o8), 2, nil)
	r.NoError(err)
	err = ct.AppendBlock()
	r.NoError(err)
	verifyBestIndex(t, ct, "a", 8, 2)

	err = ct.AppendBlock()
	r.NoError(err)
	err = ct.AppendBlock()
	r.NoError(err)
	verifyBestIndex(t, ct, "a", 8, 2)

	err = ct.ResetHeight(3)
	r.NoError(err)
	verifyBestIndex(t, ct, "A", 7, 1)
}

func TestNormalizationSortOrder(t *testing.T) {

	r := require.New(t)
	// this was an unfortunate bug; the normalization fork should not have activated anything
	// alas, it's now part of our history; we hereby test it to keep it that way
	setup(t)
	param.NormalizedNameForkHeight = 2
	ct, err := New(true)
	r.NoError(err)
	r.NotNil(ct)
	defer func() {
		err := ct.Close()
		r.NoError(err)
	}()

	hash := chainhash.HashH([]byte{1, 2, 3})

	o1 := wire.OutPoint{Hash: hash, Index: 1}
	err = ct.AddClaim([]byte("A"), o1, node.NewClaimID(o1), 1, nil)
	r.NoError(err)

	o2 := wire.OutPoint{Hash: hash, Index: 2}
	err = ct.AddClaim([]byte("A"), o2, node.NewClaimID(o2), 2, nil)
	r.NoError(err)

	o3 := wire.OutPoint{Hash: hash, Index: 3}
	err = ct.AddClaim([]byte("a"), o3, node.NewClaimID(o3), 3, nil)
	r.NoError(err)

	err = ct.AppendBlock()
	r.NoError(err)
	verifyBestIndex(t, ct, "A", 2, 2)
	verifyBestIndex(t, ct, "a", 3, 1)

	err = ct.AppendBlock()
	r.NoError(err)
	verifyBestIndex(t, ct, "a", 3, 3)
}

func verifyBestIndex(t *testing.T, ct *ClaimTrie, name string, idx uint32, claims int) {

	r := require.New(t)

	n, err := ct.nodeManager.Node([]byte(name))
	r.NoError(err)
	r.Equal(claims, len(n.Claims))
	if claims > 0 {
		r.Equal(idx, n.BestClaim.OutPoint.Index)
	}
}

func TestRebuild(t *testing.T) {
	r := require.New(t)
	setup(t)
	ct, err := New(true)
	r.NoError(err)
	r.NotNil(ct)
	defer func() {
		err := ct.Close()
		r.NoError(err)
	}()

	hash := chainhash.HashH([]byte{1, 2, 3})

	o1 := wire.OutPoint{Hash: hash, Index: 1}
	err = ct.AddClaim([]byte("test1"), o1, node.NewClaimID(o1), 1, nil)
	r.NoError(err)

	o2 := wire.OutPoint{Hash: hash, Index: 2}
	err = ct.AddClaim([]byte("test2"), o2, node.NewClaimID(o2), 2, nil)
	r.NoError(err)

	err = ct.AppendBlock()
	r.NoError(err)

	m := ct.MerkleHash()
	r.NotNil(m)
	r.NotEqual(*merkletrie.EmptyTrieHash, *m)

	ct.merkleTrie = merkletrie.NewRamTrie(ct.nodeManager)
	ct.merkleTrie.SetRoot(m, nil)

	m2 := ct.MerkleHash()
	r.NotNil(m2)
	r.Equal(*m, *m2)
}
