/*
 * Atree - Scalable Arrays and Ordered Maps
 *
 * Copyright 2021 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package atree

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/fxamacker/cbor/v2"
)

const LedgerBaseStorageSlabPrefix = "$"

type (
	Address      [8]byte
	StorageIndex [8]byte

	StorageID struct {
		Address Address
		Index   StorageIndex
	}
)

func (id StorageID) String() string {
	return fmt.Sprintf(
		"0x%x.%d",
		binary.BigEndian.Uint64(id.Address[:]),
		binary.BigEndian.Uint64(id.Index[:]),
	)
}

func (id StorageID) AddressAsUint64() uint64 {
	return binary.BigEndian.Uint64(id.Address[:])
}

func (id StorageID) IndexAsUint64() uint64 {
	return binary.BigEndian.Uint64(id.Index[:])
}

var (
	AddressUndefined      = Address{}
	StorageIndexUndefined = StorageIndex{}
	StorageIDUndefined    = StorageID{}
)

var (
	ErrStorageID    = errors.New("invalid storage id")
	ErrStorageIndex = errors.New("invalid storage index")
)

func (index StorageIndex) Next() StorageIndex {
	i := binary.BigEndian.Uint64(index[:])

	var next StorageIndex
	binary.BigEndian.PutUint64(next[:], i+1)

	return next
}

func NewStorageID(address Address, index StorageIndex) StorageID {
	return StorageID{address, index}
}

func NewStorageIDFromRawBytes(b []byte) (StorageID, error) {
	if len(b) < storageIDSize {
		return StorageID{}, fmt.Errorf("invalid storage id length %d", len(b))
	}

	var address Address
	copy(address[:], b)

	var index StorageIndex
	copy(index[:], b[8:])

	return StorageID{address, index}, nil
}

func (id StorageID) ToRawBytes(b []byte) (int, error) {
	if len(b) < storageIDSize {
		return 0, fmt.Errorf("storage id raw buffer is too short")
	}
	copy(b, id.Address[:])
	copy(b[8:], id.Index[:])
	return storageIDSize, nil
}

func (id StorageID) Valid() error {
	if id == StorageIDUndefined {
		return ErrStorageID
	}
	if id.Index == StorageIndexUndefined {
		return ErrStorageIndex
	}
	return nil
}

type BaseStorageUsageReporter interface {
	BytesRetrieved() int
	BytesStored() int
	SegmentsReturned() int
	SegmentsUpdated() int
	SegmentsTouched() int
	ResetReporter()
}

type BaseStorage interface {
	Store(StorageID, []byte) error
	Retrieve(StorageID) ([]byte, bool, error)
	Remove(StorageID) error
	GenerateStorageID(Address) (StorageID, error)
	SegmentCounts() int // number of segments stored in the storage
	Size() int          // total byte size stored
	BaseStorageUsageReporter
}

type InMemBaseStorage struct {
	segments         map[StorageID][]byte
	storageIndex     map[Address]StorageIndex
	bytesRetrieved   int
	bytesStored      int
	segmentsReturned map[StorageID]struct{}
	segmentsUpdated  map[StorageID]struct{}
	segmentsTouched  map[StorageID]struct{}
}

func NewInMemBaseStorage() *InMemBaseStorage {
	return NewInMemBaseStorageFromMap(
		make(map[StorageID][]byte),
	)
}

func NewInMemBaseStorageFromMap(segments map[StorageID][]byte) *InMemBaseStorage {
	return &InMemBaseStorage{
		segments:         segments,
		storageIndex:     make(map[Address]StorageIndex),
		segmentsReturned: make(map[StorageID]struct{}),
		segmentsUpdated:  make(map[StorageID]struct{}),
		segmentsTouched:  make(map[StorageID]struct{}),
	}
}

func (s *InMemBaseStorage) Retrieve(id StorageID) ([]byte, bool, error) {
	seg, ok := s.segments[id]
	s.bytesRetrieved += len(seg)
	s.segmentsReturned[id] = struct{}{}
	s.segmentsTouched[id] = struct{}{}
	return seg, ok, nil
}

func (s *InMemBaseStorage) Store(id StorageID, data []byte) error {
	s.segments[id] = data
	s.bytesStored += len(data)
	s.segmentsUpdated[id] = struct{}{}
	s.segmentsTouched[id] = struct{}{}
	return nil
}

func (s *InMemBaseStorage) Remove(id StorageID) error {
	s.segmentsUpdated[id] = struct{}{}
	s.segmentsTouched[id] = struct{}{}
	delete(s.segments, id)
	return nil
}

func (s *InMemBaseStorage) GenerateStorageID(address Address) (StorageID, error) {
	index := s.storageIndex[address]
	nextIndex := index.Next()

	s.storageIndex[address] = nextIndex
	return NewStorageID(address, nextIndex), nil
}

func (s *InMemBaseStorage) SegmentCounts() int {
	return len(s.segments)
}

func (s *InMemBaseStorage) Size() int {
	total := 0
	for _, seg := range s.segments {
		total += len(seg)
	}
	return total
}

func (s *InMemBaseStorage) BytesRetrieved() int {
	return s.bytesRetrieved
}

func (s *InMemBaseStorage) BytesStored() int {
	return s.bytesStored
}

func (s *InMemBaseStorage) SegmentsReturned() int {
	return len(s.segmentsReturned)
}

func (s *InMemBaseStorage) SegmentsUpdated() int {
	return len(s.segmentsUpdated)
}

func (s *InMemBaseStorage) SegmentsTouched() int {
	return len(s.segmentsTouched)
}

func (s *InMemBaseStorage) ResetReporter() {
	s.bytesStored = 0
	s.bytesRetrieved = 0
	s.segmentsReturned = make(map[StorageID]struct{})
	s.segmentsUpdated = make(map[StorageID]struct{})
	s.segmentsTouched = make(map[StorageID]struct{})
}

type Ledger interface {
	// GetValue gets a value for the given key in the storage, owned by the given account.
	GetValue(owner, key []byte) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, owned by the given account.
	SetValue(owner, key, value []byte) (err error)
	// ValueExists returns true if the given key exists in the storage, owned by the given account.
	ValueExists(owner, key []byte) (exists bool, err error)
	// AllocateStorageIndex allocates a new storage index under the given account.
	AllocateStorageIndex(owner []byte) (StorageIndex, error)
}

type LedgerBaseStorage struct {
	ledger         Ledger
	bytesRetrieved int
	bytesStored    int
}

func NewLedgerBaseStorage(ledger Ledger) *LedgerBaseStorage {
	return &LedgerBaseStorage{
		ledger:         ledger,
		bytesRetrieved: 0,
		bytesStored:    0,
	}
}

func (s *LedgerBaseStorage) Retrieve(id StorageID) ([]byte, bool, error) {
	prefixedKey := []byte(LedgerBaseStorageSlabPrefix + string(id.Index[:]))
	v, err := s.ledger.GetValue(id.Address[:], prefixedKey)
	s.bytesRetrieved += len(v)
	return v, len(v) > 0, err
}

func (s *LedgerBaseStorage) Store(id StorageID, data []byte) error {
	s.bytesStored += len(data)
	prefixedKey := []byte(LedgerBaseStorageSlabPrefix + string(id.Index[:]))
	return s.ledger.SetValue(id.Address[:], prefixedKey, data)
}

func (s *LedgerBaseStorage) Remove(id StorageID) error {
	prefixedKey := []byte(LedgerBaseStorageSlabPrefix + string(id.Index[:]))
	return s.ledger.SetValue(id.Address[:], prefixedKey, nil)
}

func (s *LedgerBaseStorage) GenerateStorageID(address Address) (StorageID, error) {
	idx, err := s.ledger.AllocateStorageIndex(address[:])
	return NewStorageID(address, idx), err
}

func (s *LedgerBaseStorage) BytesRetrieved() int {
	return s.bytesRetrieved
}

func (s *LedgerBaseStorage) BytesStored() int {
	return s.bytesStored
}

func (s *LedgerBaseStorage) SegmentCounts() int {
	// TODO
	return 0
}

func (s *LedgerBaseStorage) Size() int {
	// TODO
	return 0
}

func (s *LedgerBaseStorage) SegmentsReturned() int {
	// TODO
	return 0
}

func (s *LedgerBaseStorage) SegmentsUpdated() int {
	// TODO
	return 0
}

func (s *LedgerBaseStorage) SegmentsTouched() int {
	// TODO
	return 0
}

func (s *LedgerBaseStorage) ResetReporter() {
	s.bytesStored = 0
	s.bytesRetrieved = 0
}

type SlabStorage interface {
	Store(StorageID, Slab) error
	Retrieve(StorageID) (Slab, bool, error)
	Remove(StorageID) error
	GenerateStorageID(address Address) (StorageID, error)

	Count() int
}

type BasicSlabStorage struct {
	Slabs          map[StorageID]Slab
	storageIndex   map[Address]StorageIndex
	DecodeStorable StorableDecoder
	DecodeTypeInfo TypeInfoDecoder
	cborEncMode    cbor.EncMode
	cborDecMode    cbor.DecMode
}

var _ SlabStorage = &BasicSlabStorage{}

func NewBasicSlabStorage(
	cborEncMode cbor.EncMode,
	cborDecMode cbor.DecMode,
	decodeStorable StorableDecoder,
	decodeTypeInfo TypeInfoDecoder,
) *BasicSlabStorage {
	return &BasicSlabStorage{
		Slabs:          make(map[StorageID]Slab),
		storageIndex:   make(map[Address]StorageIndex),
		cborEncMode:    cborEncMode,
		cborDecMode:    cborDecMode,
		DecodeStorable: decodeStorable,
		DecodeTypeInfo: decodeTypeInfo,
	}
}

func (s *BasicSlabStorage) GenerateStorageID(address Address) (StorageID, error) {
	index := s.storageIndex[address]
	nextIndex := index.Next()

	s.storageIndex[address] = nextIndex
	return NewStorageID(address, nextIndex), nil
}

func (s *BasicSlabStorage) Retrieve(id StorageID) (Slab, bool, error) {
	slab, ok := s.Slabs[id]
	return slab, ok, nil
}

func (s *BasicSlabStorage) Store(id StorageID, slab Slab) error {
	s.Slabs[id] = slab
	return nil
}

func (s *BasicSlabStorage) Remove(id StorageID) error {
	delete(s.Slabs, id)
	return nil
}

func (s *BasicSlabStorage) Count() int {
	return len(s.Slabs)
}

func (s *BasicSlabStorage) StorageIDs() []StorageID {
	result := make([]StorageID, 0, len(s.Slabs))
	for storageID := range s.Slabs {
		result = append(result, storageID)
	}
	return result
}

// Encode returns serialized slabs in storage.
// This is currently used for testing.
func (s *BasicSlabStorage) Encode() (map[StorageID][]byte, error) {
	m := make(map[StorageID][]byte)
	for id, slab := range s.Slabs {
		b, err := Encode(slab, s.cborEncMode)
		if err != nil {
			return nil, err
		}
		m[id] = b
	}
	return m, nil
}

// Load deserializes encoded slabs and stores in storage.
// This is currently used for testing.
func (s *BasicSlabStorage) Load(m map[StorageID][]byte) error {
	for id, data := range m {
		slab, err := DecodeSlab(id, data, s.cborDecMode, s.DecodeStorable, s.DecodeTypeInfo)
		if err != nil {
			return err
		}
		s.Slabs[id] = slab
	}
	return nil
}

// CheckHealth checks for the health of slab storage
// it traverse the slabs and checks these factors
// - all non-root slabs only has a single parent reference (no double referencing)
// - every child of a parent shares the same ownership (childStorageID.Address == parentStorageID.Address)
// - number of root slabs are equal to the execpted number
// This should be used for testing puporses only, as it might be slow to process
func (s *BasicSlabStorage) CheckHealth(expectedNumberOfRootSlabs int) error {

	parentOf := make(map[StorageID]StorageID)
	leafs := make([]StorageID, 0)
	for id, slab := range s.Slabs {

		switch v := slab.(type) {

		case *StorableSlab:
			leafs = append(leafs, id)
		case *ArrayDataSlab:
			atLeastOneExternalSlab := false
			for _, e := range v.elements {
				if s, ok := e.(StorableSlab); ok {
					if _, found := parentOf[s.StorageID]; found {
						return fmt.Errorf("two parents are captured for the slab %s", s.StorageID)
					}
					parentOf[s.StorageID] = id
					atLeastOneExternalSlab = true
				}
			}
			if !atLeastOneExternalSlab {
				leafs = append(leafs, id)
			}

		case *ArrayMetaDataSlab:
			for _, h := range v.childrenHeaders {
				if _, found := parentOf[h.id]; found {
					return fmt.Errorf("two parents are captured for the slab %s", h.id)
				}
				parentOf[h.id] = id
			}
		case *MapDataSlab:
			atLeastOneExternalSlab := false
			elemIterator := &MapElementIterator{
				storage:  s,
				elements: v.elements,
			}
			for i := 0; i < int(v.Count()); i++ {
				keyStorable, valueStorable, err := elemIterator.Next()

				if err != nil {
					return err
				}

				if keyStorable == nil {
					break
				}

				if cid, ok := keyStorable.(StorageIDStorable); ok {
					if _, found := parentOf[StorageID(cid)]; found {
						return fmt.Errorf("two parents are captured for the slab %s", StorageID(cid))
					}
					parentOf[StorageID(cid)] = id
					atLeastOneExternalSlab = true
				}

				if cid, ok := valueStorable.(StorageIDStorable); ok {
					if _, found := parentOf[StorageID(cid)]; found {
						return fmt.Errorf("two parents are captured for the slab %s", StorageID(cid))
					}
					parentOf[StorageID(cid)] = id
					atLeastOneExternalSlab = true
				}
			}
			if !atLeastOneExternalSlab {
				leafs = append(leafs, id)
			}
		case *MapMetaDataSlab:
			for _, h := range v.childrenHeaders {
				if _, found := parentOf[h.id]; found {
					return fmt.Errorf("two parents are captured for the slab %s", h.id)
				}
				parentOf[h.id] = id
			}
		default:
			return fmt.Errorf("unknown type of slab %T", slab)
		}
	}

	roots := make([]StorageID, 0)
	rootsMap := make(map[StorageID]bool)
	visited := make(map[StorageID]bool)
	var id StorageID
	for _, leaf := range leafs {
		id = leaf
		if visited[id] {
			return fmt.Errorf("atleast two references found to the leaf slab %s", id)
		}
		visited[id] = true
		for {
			p, found := parentOf[id]
			if !found {
				// we reach the root
				rootsMap[id] = true
				break
			}
			visited[p] = true
			if s.Slabs[id].ID().Address != s.Slabs[p].ID().Address {
				return fmt.Errorf("parent and child are not owned by the same account child.owner: %s, parent.owner: %s", s.Slabs[id].ID().Address, s.Slabs[p].ID().Address)
			}
			id = p
		}
	}

	if len(visited) != len(s.Slabs) {
		return fmt.Errorf("an slab was not reachable from leafs - broken connection somewhere - number of slabs:%d, visited during traverse: %d", len(s.Slabs), len(visited))
	}

	for k := range rootsMap {
		roots = append(roots, k)
	}

	if len(roots) != expectedNumberOfRootSlabs {
		return fmt.Errorf("number of root slabs doesn't match expected: %d, got: %d", expectedNumberOfRootSlabs, len(roots))

	}

	return nil
}

type PersistentSlabStorage struct {
	baseStorage      BaseStorage
	cache            map[StorageID]Slab
	deltas           map[StorageID]Slab
	tempStorageIndex uint64
	DecodeStorable   StorableDecoder
	DecodeTypeInfo   TypeInfoDecoder
	cborEncMode      cbor.EncMode
	cborDecMode      cbor.DecMode
	autoCommit       bool // flag to call commit after each operation
}

var _ SlabStorage = &PersistentSlabStorage{}

type StorageOption func(st *PersistentSlabStorage) *PersistentSlabStorage

func NewPersistentSlabStorage(
	base BaseStorage,
	cborEncMode cbor.EncMode,
	cborDecMode cbor.DecMode,
	decodeStorable StorableDecoder,
	decodeTypeInfo TypeInfoDecoder,
	opts ...StorageOption,
) *PersistentSlabStorage {
	storage := &PersistentSlabStorage{baseStorage: base,
		cache:          make(map[StorageID]Slab),
		deltas:         make(map[StorageID]Slab),
		cborEncMode:    cborEncMode,
		cborDecMode:    cborDecMode,
		DecodeStorable: decodeStorable,
		DecodeTypeInfo: decodeTypeInfo,
		autoCommit:     false,
	}

	for _, applyOption := range opts {
		storage = applyOption(storage)
	}

	return storage
}

// WithAutoCommit sets the autocommit functionality
func WithAutoCommit() StorageOption {
	return func(st *PersistentSlabStorage) *PersistentSlabStorage {
		st.autoCommit = true
		return st
	}
}

func (s *PersistentSlabStorage) GenerateStorageID(address Address) (StorageID, error) {
	if address == AddressUndefined {
		var idx StorageIndex
		s.tempStorageIndex++
		binary.BigEndian.PutUint64(idx[:], s.tempStorageIndex)
		return NewStorageID(address, idx), nil
	}
	return s.baseStorage.GenerateStorageID(address)
}

func (s *PersistentSlabStorage) sortedOwnedDeltaKeys() []StorageID {
	keysWithOwners := make([]StorageID, 0, len(s.deltas))
	for k := range s.deltas {
		// ignore the ones that are not owned by accounts
		if k.Address != AddressUndefined {
			keysWithOwners = append(keysWithOwners, k)
		}
	}

	sort.Slice(keysWithOwners, func(i, j int) bool {
		a := keysWithOwners[i]
		b := keysWithOwners[j]
		if a.Address == b.Address {
			return a.IndexAsUint64() < b.IndexAsUint64()
		}
		return a.AddressAsUint64() < b.AddressAsUint64()
	})
	return keysWithOwners
}

func (s *PersistentSlabStorage) Commit() error {
	var err error

	// this part ensures the keys are sorted so commit operation is deterministic
	keysWithOwners := s.sortedOwnedDeltaKeys()

	for _, id := range keysWithOwners {
		slab := s.deltas[id]

		// deleted slabs
		if slab == nil {
			err = s.baseStorage.Remove(id)
			if err != nil {
				return err
			}
			continue
		}

		// serialize
		data, err := Encode(slab, s.cborEncMode)
		if err != nil {
			return err
		}

		// store
		err = s.baseStorage.Store(id, data)
		if err != nil {
			return err
		}

		// add to read cache
		s.cache[id] = slab
	}
	// reset deltas
	s.deltas = make(map[StorageID]Slab)
	return nil
}

func (s *PersistentSlabStorage) FastCommit(numWorkers int) error {

	// this part ensures the keys are sorted so commit operation is deterministic
	keysWithOwners := s.sortedOwnedDeltaKeys()

	// construct job queue
	jobs := make(chan StorageID, len(keysWithOwners))
	defer close(jobs)
	for _, id := range keysWithOwners {
		jobs <- id
	}

	type encodedSlabs struct {
		storageID StorageID
		data      []byte
		err       error
	}

	// construct result queue
	results := make(chan *encodedSlabs, len(keysWithOwners))
	defer close(results)

	// define encoders (workers) and launch them
	// encoders encodes slabs in parallel
	encoder := func(jobs <-chan StorageID, results chan<- *encodedSlabs) {
		for id := range jobs {
			slab := s.deltas[id]
			if slab == nil {
				results <- &encodedSlabs{
					storageID: id,
					data:      nil,
					err:       nil,
				}
				continue
			}
			// serialize
			data, err := Encode(slab, s.cborEncMode)
			results <- &encodedSlabs{
				storageID: id,
				data:      data,
				err:       err,
			}
		}
	}

	for i := 0; i < numWorkers; i++ {
		go encoder(jobs, results)
	}

	// process the results while encoders are working
	// we need to capture them inside a map
	// again so we can apply them in order of keys
	encSlabByID := make(map[StorageID][]byte)
	for i := 0; i < len(keysWithOwners); i++ {
		result := <-results
		// if any error return
		if result.err != nil {
			return result.err
		}
		encSlabByID[result.storageID] = result.data
	}

	// at this stage all results has been processed
	// and ready to be passed to base storage layer
	for _, id := range keysWithOwners {
		data := encSlabByID[id]

		var err error
		// deleted slabs
		if data == nil {
			err = s.baseStorage.Remove(id)
			if err != nil {
				return err
			}
			continue
		}

		// store
		err = s.baseStorage.Store(id, data)
		if err != nil {
			return err
		}

		// TODO: we might skip this since cadence
		// never uses the storage after commit
		// add to read cache
		s.cache[id] = s.deltas[id]
	}

	// reset deltas
	s.deltas = make(map[StorageID]Slab)
	return nil
}

func (s *PersistentSlabStorage) DropDeltas() {
	s.deltas = make(map[StorageID]Slab)
}

func (s *PersistentSlabStorage) DropCache() {
	s.cache = make(map[StorageID]Slab)
}

func (s *PersistentSlabStorage) Retrieve(id StorageID) (Slab, bool, error) {
	var slab Slab

	// check deltas first
	if slab, ok := s.deltas[id]; ok {
		return slab, slab != nil, nil
	}

	// check the read cache next
	if slab, ok := s.cache[id]; ok {
		return slab, true, nil
	}

	// fetch from base storage last
	data, ok, err := s.baseStorage.Retrieve(id)
	if err != nil {
		return nil, false, err
	}
	slab, err = DecodeSlab(id, data, s.cborDecMode, s.DecodeStorable, s.DecodeTypeInfo)
	if err == nil {
		// save decoded slab to cache
		s.cache[id] = slab
	}
	return slab, ok, err
}

func (s *PersistentSlabStorage) Store(id StorageID, slab Slab) error {
	if s.autoCommit {
		data, err := Encode(slab, s.cborEncMode)
		if err != nil {
			return err
		}
		err = s.baseStorage.Store(id, data)
		if err != nil {
			return err
		}
		s.cache[id] = slab
		return nil
	}

	// add to deltas
	s.deltas[id] = slab
	return nil
}

func (s *PersistentSlabStorage) Remove(id StorageID) error {
	if s.autoCommit {
		err := s.baseStorage.Remove(id)
		if err != nil {
			return err
		}
	}

	// add to nil to deltas under that id
	s.deltas[id] = nil
	return nil
}

// Warning Counts doesn't consider new segments in the deltas and only returns commited values
func (s *PersistentSlabStorage) Count() int {
	return s.baseStorage.SegmentCounts()
}
