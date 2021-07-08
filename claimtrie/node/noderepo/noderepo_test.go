package noderepo

import (
	"testing"

	"github.com/btcsuite/btcd/claimtrie/node"

	"github.com/stretchr/testify/require"
)

var (
	opStr1        = "0000000000000000000000000000000000000000000000000000000000000000:1"
	testNodeName1 = []byte("name1")
)

func TestPebble(t *testing.T) {

	r := require.New(t)

	repo, err := NewPebble(t.TempDir())
	r.NoError(err)
	defer func() {
		err := repo.Close()
		r.NoError(err)
	}()

	cleanup := func() {
		lowerBound := testNodeName1
		upperBound := append(testNodeName1, byte(0))
		err := repo.db.DeleteRange(lowerBound, upperBound, nil)
		r.NoError(err)
	}

	testNodeRepo(t, repo, func() {}, cleanup)
}

func testNodeRepo(t *testing.T, repo node.Repo, setup, cleanup func()) {

	r := require.New(t)

	chg := node.NewChange(node.AddClaim).SetName(testNodeName1).SetOutPoint(opStr1)

	testcases := []struct {
		name     string
		height   int32
		changes  []node.Change
		expected []node.Change
	}{
		{
			"test 1",
			1,
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
			[]node.Change{chg.SetHeight(1)},
		},
		{
			"test 2",
			2,
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
			[]node.Change{chg.SetHeight(1)},
		},
		{
			"test 3",
			3,
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3)},
		},
		{
			"test 4",
			4,
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3)},
		},
		{
			"test 5",
			5,
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
		},
		{
			"test 6",
			6,
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
			[]node.Change{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
		},
	}

	for _, tt := range testcases {

		setup()

		err := repo.AppendChanges(tt.changes)
		r.NoError(err)

		changes, err := repo.LoadChanges(testNodeName1)
		r.NoError(err)
		r.Equalf(tt.expected, changes[:len(tt.expected)], tt.name)

		cleanup()
	}

	testcases2 := []struct {
		name     string
		height   int32
		changes  [][]node.Change
		expected []node.Change
	}{
		{
			"Save in 2 batches, and load up to 1",
			1,
			[][]node.Change{
				{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
				{chg.SetHeight(6), chg.SetHeight(8), chg.SetHeight(9)},
			},
			[]node.Change{chg.SetHeight(1)},
		},
		{
			"Save in 2 batches, and load up to 9",
			9,
			[][]node.Change{
				{chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5)},
				{chg.SetHeight(6), chg.SetHeight(8), chg.SetHeight(9)},
			},
			[]node.Change{
				chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5),
				chg.SetHeight(6), chg.SetHeight(8), chg.SetHeight(9),
			},
		},
		{
			"Save in 3 batches, and load up to 8",
			8,
			[][]node.Change{
				{chg.SetHeight(1), chg.SetHeight(3)},
				{chg.SetHeight(5)},
				{chg.SetHeight(6), chg.SetHeight(8), chg.SetHeight(9)},
			},
			[]node.Change{
				chg.SetHeight(1), chg.SetHeight(3), chg.SetHeight(5),
				chg.SetHeight(6), chg.SetHeight(8),
			},
		},
	}

	for _, tt := range testcases2 {

		setup()

		for _, changes := range tt.changes {
			err := repo.AppendChanges(changes)
			r.NoError(err)
		}

		changes, err := repo.LoadChanges(testNodeName1)
		r.NoError(err)
		r.Equalf(tt.expected, changes[:len(tt.expected)], tt.name)

		cleanup()
	}
}

func TestIterator(t *testing.T) {

	r := require.New(t)

	repo, err := NewPebble(t.TempDir())
	r.NoError(err)
	defer func() {
		err := repo.Close()
		r.NoError(err)
	}()

	creation := []node.Change{
		{Name: []byte("test\x00"), Height: 5},
		{Name: []byte("test\x00\x00"), Height: 5},
		{Name: []byte("test\x00b"), Height: 5},
		{Name: []byte("test\x00\xFF"), Height: 5},
		{Name: []byte("testa"), Height: 5},
	}
	err = repo.AppendChanges(creation)
	r.NoError(err)

	var received []node.Change
	repo.IterateChildren([]byte{}, func(changes []node.Change) bool {
		received = append(received, changes...)
		return true
	})
	r.Equal(creation, received)
}
