/*
 * Copyright 2021 Dapper Labs, Inc.  All rights reserved.
 */

package atree

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

type Stats struct {
	Levels            uint64
	ElementCount      uint64
	MetaDataSlabCount uint64
	DataSlabCount     uint64
}

// ArrayStats returns stats about array slabs.
func ArrayStats(a *Array) (Stats, error) {
	level := uint64(0)
	metaDataSlabCount := uint64(0)
	metaDataSlabSize := uint64(0)
	dataSlabCount := uint64(0)
	dataSlabSize := uint64(0)

	nextLevelIDs := list.New()
	nextLevelIDs.PushBack(a.root.Header().id)

	for nextLevelIDs.Len() > 0 {

		ids := nextLevelIDs

		nextLevelIDs = list.New()

		for e := ids.Front(); e != nil; e = e.Next() {
			id := e.Value.(StorageID)

			slab, err := getArraySlab(a.Storage, id)
			if err != nil {
				return Stats{}, err
			}

			if slab.IsData() {
				leaf := slab.(*ArrayDataSlab)
				dataSlabCount++
				dataSlabSize += uint64(leaf.header.size)
			} else {
				meta := slab.(*ArrayMetaDataSlab)
				metaDataSlabCount++
				metaDataSlabSize += uint64(meta.header.size)

				for _, h := range meta.childrenHeaders {
					nextLevelIDs.PushBack(h.id)
				}
			}
		}

		level++
	}

	return Stats{
		Levels:            level,
		ElementCount:      a.Count(),
		MetaDataSlabCount: metaDataSlabCount,
		DataSlabCount:     dataSlabCount,
	}, nil
}

// PrintArray prints array slab data to stdout.
func PrintArray(a *Array) {
	nextLevelIDs := list.New()
	nextLevelIDs.PushBack(a.root.Header().id)

	overflowIDs := list.New()

	level := 0
	for nextLevelIDs.Len() > 0 {

		ids := nextLevelIDs

		nextLevelIDs = list.New()

		for e := ids.Front(); e != nil; e = e.Next() {
			id := e.Value.(StorageID)

			slab, err := getArraySlab(a.Storage, id)
			if err != nil {
				fmt.Println(err)
				return
			}

			if slab.IsData() {
				dataSlab := slab.(*ArrayDataSlab)
				fmt.Printf("level %d, leaf (%+v): ", level+1, dataSlab.header)

				var elements []Storable
				if len(dataSlab.elements) <= 6 {
					elements = dataSlab.elements
				} else {
					elements = append(elements, dataSlab.elements[:3]...)
					elements = append(elements, dataSlab.elements[len(dataSlab.elements)-3:]...)
				}

				var elemsStr []string
				for _, e := range elements {
					if id, ok := e.(StorageIDStorable); ok {
						overflowIDs.PushBack(StorageID(id))
					}
					elemsStr = append(elemsStr, fmt.Sprint(e))
				}

				if len(dataSlab.elements) > 6 {
					elemsStr = append(elemsStr, "")
					copy(elemsStr[4:], elemsStr[3:])
					elemsStr[3] = "..."
				}
				fmt.Printf("[%s]\n", strings.Join(elemsStr, " "))
			} else {
				meta := slab.(*ArrayMetaDataSlab)
				fmt.Printf("level %d, meta (%+v) headers: [", level+1, meta.header)
				for _, h := range meta.childrenHeaders {
					fmt.Printf("%+v ", h)
					nextLevelIDs.PushBack(h.id)
				}
				fmt.Println("]")
			}
		}

		level++
	}

	if overflowIDs.Len() > 0 {
		for e := overflowIDs.Front(); e != nil; e = e.Next() {
			id := e.Value.(StorageID)

			// TODO: expand this to include other types
			slab, err := getArraySlab(a.Storage, id)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			fmt.Printf("overflow: (id %d) %s\n", id, slab.String())
		}
	}
}

func (a *Array) valid(typeInfo cbor.RawMessage) (bool, error) {

	// Verify that root has type information
	extraData := a.root.ExtraData()
	if extraData == nil {
		return false, errors.New("root slab doesn't have extra data")
	}
	if !bytes.Equal(extraData.TypeInfo, typeInfo) {
		return false, fmt.Errorf(
			"type information is %v, want %v",
			extraData.TypeInfo,
			typeInfo,
		)
	}

	verified, _, err := a._valid(a.root.Header().id, 0)
	return verified, err
}

func (a *Array) _valid(id StorageID, level int) (bool, uint32, error) {

	slab, err := getArraySlab(a.Storage, id)
	if err != nil {
		return false, 0, err
	}

	if level > 0 {
		if slab.ExtraData() != nil {
			return false, 0, errors.New("non-root slab has extra data")
		}
	}

	if slab.IsData() {
		dataSlab, ok := slab.(*ArrayDataSlab)
		if !ok {
			return false, 0, fmt.Errorf("slab %d is not ArrayDataSlab", id)
		}

		count := uint32(len(dataSlab.elements))

		computedSize := uint32(0)
		for _, e := range dataSlab.elements {
			computedSize += e.ByteSize()
		}

		_, underflow := dataSlab.IsUnderflow()
		validFill := (level == 0) || (!dataSlab.IsFull() && !underflow)

		validCount := count == dataSlab.header.count

		validSize := (arrayDataSlabPrefixSize + computedSize) == dataSlab.header.size

		return validFill && validCount && validSize, count, nil
	}

	meta, ok := slab.(*ArrayMetaDataSlab)
	if !ok {
		return false, 0, fmt.Errorf("slab %d is not ArrayMetaDataSlab", id)
	}
	sum := uint32(0)
	for _, h := range meta.childrenHeaders {
		verified, count, err := a._valid(h.id, level+1)
		if !verified || err != nil {
			return false, 0, err
		}
		sum += count
	}

	_, underflow := meta.IsUnderflow()
	validFill := (level == 0) || (!meta.IsFull() && !underflow)

	validCount := sum == meta.header.count

	computedSize := uint32(len(meta.childrenHeaders)*arraySlabHeaderSize) + arrayMetaDataSlabPrefixSize
	validSize := computedSize == meta.header.size

	return validFill && validCount && validSize, sum, nil
}
