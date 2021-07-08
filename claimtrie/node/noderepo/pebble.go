package noderepo

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"

	"github.com/btcsuite/btcd/claimtrie/node"
	"github.com/cockroachdb/pebble"
	"github.com/vmihailenco/msgpack/v5"
)

type Pebble struct {
	db *pebble.DB
}

func init() {
	claimEncoder := func (e *msgpack.Encoder, v reflect.Value) error {
		claim := v.Interface().(node.ClaimID)
		return e.EncodeString(claim.String())
	}
	claimDecoder := func (e *msgpack.Decoder, v reflect.Value) error {
		s, err := e.DecodeString()
		if err != nil {
			return err
		}
		id, err := node.NewIDFromString(s)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(id))
		return nil
	}
	msgpack.Register(node.ClaimID{}, claimEncoder, claimDecoder)
}

func NewPebble(path string) (*Pebble, error) {

	db, err := pebble.Open(path, &pebble.Options{Cache: pebble.NewCache(128 << 20), BytesPerSync: 16 << 20})
	if err != nil {
		return nil, fmt.Errorf("pebble open %s, %w", path, err)
	}

	repo := &Pebble{db: db}

	return repo, nil
}

// AppendChanges makes an assumption that anything you pass to it is newer than what was saved before.
func (repo *Pebble) AppendChanges(changes []node.Change) error {

	batch := repo.db.NewBatch()

	// TODO: switch to buffer pool and reuse encoder
	for _, chg := range changes {
		value, err := msgpack.Marshal(chg)
		if err != nil {
			return fmt.Errorf("msgpack marshal value: %w", err)
		}

		err = batch.Merge(chg.Name, value, pebble.NoSync)
		if err != nil {
			return fmt.Errorf("pebble set: %w", err)
		}
	}
	err := batch.Commit(pebble.NoSync)
	if err != nil {
		return fmt.Errorf("pebble save commit: %w", err)
	}
	batch.Close()
	return err
}

func (repo *Pebble) LoadChanges(name []byte) ([]node.Change, error) {

	data, closer, err := repo.db.Get(name)
	if err != nil && err != pebble.ErrNotFound {
		return nil, fmt.Errorf("pebble get: %w", err)
	}
	if closer != nil {
		defer closer.Close()
	}

	return unmarshalChanges(data)
}

func unmarshalChanges(data []byte) ([]node.Change, error) {
	var changes []node.Change
	dec := msgpack.GetDecoder()
	defer msgpack.PutDecoder(dec)

	reader := bytes.NewReader(data)
	dec.Reset(reader)
	for reader.Len() > 0 {
		var chg node.Change
		err := dec.Decode(&chg)
		if err != nil {
			return nil, fmt.Errorf("msgpack unmarshal: %w", err)
		}
		changes = append(changes, chg)
	}

	// this was required for the normalization stuff:
	sort.SliceStable(changes, func(i, j int) bool {
		return changes[i].Height < changes[j].Height
	})

	return changes, nil
}

func (repo *Pebble) DropChanges(name []byte, finalHeight int32) error {
	changes, err := repo.LoadChanges(name)
	i := 0
	for ; i < len(changes); i++ {
		if changes[i].Height > finalHeight {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("pebble drop: %w", err)
	}
	// making a performance assumption that DropChanges won't happen often:
	err = repo.db.Set(name, []byte{}, pebble.NoSync)
	if err != nil {
		return fmt.Errorf("pebble drop: %w", err)
	}
	return repo.AppendChanges(changes[:i])
}

func (repo *Pebble) IterateChildren(name []byte, f func(changes []node.Change) bool) {
	end := bytes.NewBuffer(nil)
	end.Write(name)
	end.Write(bytes.Repeat([]byte{255, 255, 255, 255}, 64))

	prefixIterOptions := &pebble.IterOptions{
		LowerBound: name,
		UpperBound: end.Bytes(),
	}

	iter := repo.db.NewIter(prefixIterOptions)
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		changes, err := unmarshalChanges(iter.Value())
		if err != nil {
			panic(err)
		}
		if !f(changes) {
			return
		}
	}
}

func (repo *Pebble) IterateAll(predicate func(name []byte) bool) {
	iter := repo.db.NewIter(nil)
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if !predicate(iter.Key()) {
			break
		}
	}
}

func (repo *Pebble) Close() error {

	err := repo.db.Flush()
	if err != nil {
		return fmt.Errorf("pebble flush: %w", err)
	}

	err = repo.db.Close()
	if err != nil {
		return fmt.Errorf("pebble close: %w", err)
	}

	return nil
}
