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
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockDigesterBuilder struct {
	mock.Mock
}

var _ DigesterBuilder = &mockDigesterBuilder{}

type mockDigester struct {
	d []Digest
}

var _ Digester = &mockDigester{}

func (h *mockDigesterBuilder) SetSeed(_ uint64, _ uint64) {
}

func (h *mockDigesterBuilder) Digest(hip HashInputProvider, value Value) (Digester, error) {
	args := h.Called(value)
	return args.Get(0).(mockDigester), nil
}

func (d mockDigester) DigestPrefix(level int) ([]Digest, error) {
	if level > len(d.d) {
		return nil, fmt.Errorf("digest level %d out of bounds", level)
	}
	return d.d[:level], nil
}

func (d mockDigester) Digest(level int) (Digest, error) {
	if level >= len(d.d) {
		return 0, fmt.Errorf("digest level %d out of bounds", level)
	}
	return d.d[level], nil
}

func (d mockDigester) Levels() int {
	return len(d.d)
}

func (d mockDigester) Reset() {}

func newTestInMemoryStorage(t testing.TB) *BasicSlabStorage {

	encMode, err := cbor.EncOptions{}.EncMode()
	require.NoError(t, err)

	decMode, err := cbor.DecOptions{}.DecMode()
	require.NoError(t, err)

	storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

	return storage
}

// TODO: use newTestPersistentStorage after serialization is implemented.
func TestMapSetAndGet(t *testing.T) {

	t.Run("unique keys", func(t *testing.T) {

		const mapSize = 64 * 1024

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		storage := newTestInMemoryStorage(t)

		uniqueKeys := make(map[string]bool, mapSize)
		uniqueKeyValues := make(map[Value]Value, mapSize)
		for i := uint64(0); i < mapSize; i++ {
			for {
				s := randStr(16)
				if !uniqueKeys[s] {
					uniqueKeys[s] = true

					k := NewStringValue(s)
					uniqueKeyValues[k] = Uint64Value(i)
					break
				}
			}
		}

		m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			e, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, e)
		}

		require.Equal(t, typeInfo, m.Type())
		require.Equal(t, uint64(len(uniqueKeyValues)), m.Count())

		stats, _ := GetMapStats(m)
		require.Equal(t, stats.DataSlabCount+stats.MetaDataSlabCount+stats.CollisionDataSlabCount, uint64(m.Storage.Count()))
	})

	t.Run("replicate keys", func(t *testing.T) {

		const mapSize = 64 * 1024

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		storage := newTestInMemoryStorage(t)

		uniqueKeys := make(map[string]bool, mapSize)
		uniqueKeyValues := make(map[Value]Value, mapSize)
		for i := uint64(0); i < mapSize; i++ {
			for {
				s := randStr(16)
				if !uniqueKeys[s] {
					uniqueKeys[s] = true

					k := NewStringValue(s)
					uniqueKeyValues[k] = Uint64Value(i)
					break
				}
			}
		}

		m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		// Overwrite previously inserted values
		for k := range uniqueKeyValues {
			oldv, _ := uniqueKeyValues[k].(Uint64Value)
			v := Uint64Value(uint64(oldv) + mapSize)
			uniqueKeyValues[k] = v

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.NotNil(t, existingStorable)

			existingValue, err := existingStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, oldv, existingValue)
		}

		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			e, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, e)
		}

		require.Equal(t, typeInfo, m.Type())
		require.Equal(t, uint64(len(uniqueKeyValues)), m.Count())

		stats, _ := GetMapStats(m)
		require.Equal(t, stats.DataSlabCount+stats.MetaDataSlabCount+stats.CollisionDataSlabCount, uint64(m.Storage.Count()))
	})

	// Test random string with random length as key and random uint as value
	t.Run("random key and value", func(t *testing.T) {

		const mapSize = 64 * 1024
		const maxKeyLength = 224

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		uniqueKeys := make(map[string]bool, mapSize)
		uniqueKeyValues := make(map[Value]Value, mapSize)
		for i := uint64(0); i < mapSize; i++ {
			for {
				slen := rand.Intn(maxKeyLength + 1)
				s := randStr(slen)

				if !uniqueKeys[s] {
					uniqueKeys[s] = true

					k := NewStringValue(s)
					uniqueKeyValues[k] = RandomValue()
					break
				}
			}
		}

		storage := newTestInMemoryStorage(t)

		m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			e, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, e)
		}

		require.Equal(t, typeInfo, m.Type())
		require.Equal(t, uint64(len(uniqueKeyValues)), m.Count())

		stats, _ := GetMapStats(m)
		require.Equal(t, stats.DataSlabCount+stats.MetaDataSlabCount+stats.CollisionDataSlabCount, uint64(m.Storage.Count()))
	})
}

func TestMapHash(t *testing.T) {

	const mapSize = 64 * 1024

	typeInfo := testTypeInfo{42}

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	storage := newTestInMemoryStorage(t)

	uniqueKeys := make(map[string]bool, mapSize*2)
	var keysToInsert []string
	var keysToNotInsert []string
	for i := uint64(0); i < mapSize*2; i++ {
		for {
			s := randStr(16)
			if !uniqueKeys[s] {
				uniqueKeys[s] = true

				if i%2 == 0 {
					keysToInsert = append(keysToInsert, s)
				} else {
					keysToNotInsert = append(keysToNotInsert, s)
				}

				break
			}
		}
	}

	m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
	require.NoError(t, err)

	for i, k := range keysToInsert {
		existingStorable, err := m.Set(compare, hashInputProvider, NewStringValue(k), Uint64Value(i))
		require.NoError(t, err)
		require.Nil(t, existingStorable)
	}

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	for _, k := range keysToInsert {
		exist, err := m.Has(compare, hashInputProvider, NewStringValue(k))
		require.NoError(t, err)
		require.Equal(t, true, exist)
	}

	for _, k := range keysToNotInsert {
		exist, err := m.Has(compare, hashInputProvider, NewStringValue(k))
		require.NoError(t, err)
		require.Equal(t, false, exist)
	}

	require.Equal(t, typeInfo, m.Type())
	require.Equal(t, uint64(mapSize), m.Count())
}

func TestMapRemove(t *testing.T) {

	SetThreshold(512)
	defer func() {
		SetThreshold(1024)
	}()

	t.Run("small key and value", func(t *testing.T) {

		const mapSize = 2 * 1024

		const keyStringMaxSize = 16

		const valueStringMaxSize = 16

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		storage := newTestInMemoryStorage(t)

		uniqueKeys := make(map[string]bool, mapSize)
		uniqueKeyValues := make(map[Value]Value, mapSize)
		for i := uint64(0); i < mapSize; i++ {
			for {
				s := randStr(keyStringMaxSize)
				if !uniqueKeys[s] {
					uniqueKeys[s] = true

					k := NewStringValue(s)
					uniqueKeyValues[k] = NewStringValue(randStr(valueStringMaxSize))
					break
				}
			}
		}

		m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
		require.NoError(t, err)

		// Insert elements
		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		// Get elements
		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			e, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, e)
		}

		count := len(uniqueKeyValues)

		// Remove all elements
		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			removedKeyStorable, removedValueStorable, err := m.Remove(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			removedKey, err := removedKeyStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, k, removedKey)

			removedValue, err := removedValueStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, removedValue)

			removedKeyStorable, removedValueStorable, err = m.Remove(compare, hashInputProvider, NewStringValue(strv.str))
			require.Error(t, err)
			require.Nil(t, removedKeyStorable)
			require.Nil(t, removedValueStorable)

			count--

			require.Equal(t, uint64(count), m.Count())

			require.Equal(t, typeInfo, m.Type())
		}

		stats, _ := GetMapStats(m)
		require.Equal(t, uint64(1), stats.DataSlabCount)
		require.Equal(t, uint64(0), stats.MetaDataSlabCount)
		require.Equal(t, uint64(0), stats.CollisionDataSlabCount)

		require.Equal(t, int(1), m.Storage.Count())
	})

	t.Run("large key and value", func(t *testing.T) {
		const mapSize = 2 * 1024

		const keyStringMaxSize = 512

		const valueStringMaxSize = 512

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		storage := newTestInMemoryStorage(t)

		uniqueKeys := make(map[string]bool, mapSize)
		uniqueKeyValues := make(map[Value]Value, mapSize)
		for i := uint64(0); i < mapSize; i++ {
			for {
				s := randStr(keyStringMaxSize)
				if !uniqueKeys[s] {
					uniqueKeys[s] = true

					k := NewStringValue(s)
					uniqueKeyValues[k] = NewStringValue(randStr(valueStringMaxSize))
					break
				}
			}
		}

		m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
		require.NoError(t, err)

		// Insert elements
		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		// Get elements
		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			e, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, e)
		}

		count := len(uniqueKeyValues)

		// Remove all elements
		for k, v := range uniqueKeyValues {
			strv := k.(StringValue)
			require.NotNil(t, strv)

			removedKeyStorable, removedValueStorable, err := m.Remove(compare, hashInputProvider, NewStringValue(strv.str))
			require.NoError(t, err)

			removedKey, err := removedKeyStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, k, removedKey)

			removedValue, err := removedValueStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, v, removedValue)

			removedKeyStorable, removedValueStorable, err = m.Remove(compare, hashInputProvider, NewStringValue(strv.str))
			require.Error(t, err)
			require.Nil(t, removedKeyStorable)
			require.Nil(t, removedValueStorable)

			count--

			require.Equal(t, uint64(count), m.Count())

			require.Equal(t, typeInfo, m.Type())
		}

		stats, _ := GetMapStats(m)
		require.Equal(t, uint64(1), stats.DataSlabCount)
		require.Equal(t, uint64(0), stats.MetaDataSlabCount)
		require.Equal(t, uint64(0), stats.CollisionDataSlabCount)
	})
}

func TestMapIterate(t *testing.T) {
	t.Run("no collision", func(t *testing.T) {
		const mapSize = 64 * 1024

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		storage := newTestInMemoryStorage(t)

		digesterBuilder := newBasicDigesterBuilder()

		uniqueKeyValues := make(map[string]uint64, mapSize)

		sortedKeys := make([]StringValue, mapSize)

		for i := uint64(0); i < mapSize; i++ {
			for {
				s := randStr(16)
				if _, exist := uniqueKeyValues[s]; !exist {
					sortedKeys[i] = NewStringValue(s)
					uniqueKeyValues[s] = i
					break
				}
			}
		}

		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, NewStringValue(k), Uint64Value(v))
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		// Sort keys by hashed value
		sort.SliceStable(sortedKeys, func(i, j int) bool {
			d1, err := digesterBuilder.Digest(hashInputProvider, sortedKeys[i])
			require.NoError(t, err)

			digest1, err := d1.DigestPrefix(d1.Levels())
			require.NoError(t, err)

			d2, err := digesterBuilder.Digest(hashInputProvider, sortedKeys[j])
			require.NoError(t, err)

			digest2, err := d2.DigestPrefix(d2.Levels())
			require.NoError(t, err)

			for z := 0; z < len(digest1); z++ {
				if digest1[z] != digest2[z] {
					return digest1[z] < digest2[z] // sort by hkey
				}
			}
			return i < j // sort by insertion order with hash collision
		})

		// Iterate key value pairs
		i := uint64(0)
		err = m.Iterate(func(k Value, v Value) (resume bool, err error) {
			ks, ok := k.(StringValue)
			require.True(t, ok)
			require.Equal(t, sortedKeys[i].String(), ks.String())

			vi, ok := v.(Uint64Value)
			require.True(t, ok)
			require.Equal(t, uniqueKeyValues[ks.String()], uint64(vi))

			i++

			return true, nil
		})

		require.NoError(t, err)
		require.Equal(t, i, uint64(mapSize))

		// Iterate keys
		i = uint64(0)
		err = m.IterateKeys(func(k Value) (resume bool, err error) {
			ks, ok := k.(StringValue)
			require.True(t, ok)
			require.Equal(t, sortedKeys[i].String(), ks.String())

			i++

			return true, nil
		})

		require.NoError(t, err)
		require.Equal(t, i, uint64(mapSize))

		// Iterate values
		i = uint64(0)
		err = m.IterateValues(func(v Value) (resume bool, err error) {
			key := sortedKeys[i]

			vi, ok := v.(Uint64Value)
			require.True(t, ok)
			require.Equal(t, uniqueKeyValues[key.str], uint64(vi))

			i++

			return true, nil
		})

		require.NoError(t, err)
		require.Equal(t, i, uint64(mapSize))

		require.Equal(t, typeInfo, m.Type())
	})

	t.Run("collision", func(t *testing.T) {
		const mapSize = 1024

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		digesterBuilder := &mockDigesterBuilder{}

		storage := newTestInMemoryStorage(t)

		uniqueKeyValues := make(map[Value]Value, mapSize)

		uniqueKeys := make(map[string]bool, mapSize)

		sortedKeys := make([]Value, mapSize)

		// keys is needed to insert elements with collision in deterministic order.
		keys := make([]Value, mapSize)

		for i := uint64(0); i < mapSize; i++ {
			for {
				s := randStr(16)

				if !uniqueKeys[s] {
					uniqueKeys[s] = true

					k := NewStringValue(s)
					v := NewStringValue(randStr(16))

					sortedKeys[i] = k
					keys[i] = k
					uniqueKeyValues[k] = v

					digests := []Digest{
						Digest(rand.Intn(256)),
						Digest(rand.Intn(256)),
						Digest(rand.Intn(256)),
						Digest(rand.Intn(256)),
					}

					digesterBuilder.On("Digest", k).Return(mockDigester{digests})
					break
				}
			}
		}

		// Sort keys by hashed value
		sort.SliceStable(sortedKeys, func(i, j int) bool {

			d1, err := digesterBuilder.Digest(hashInputProvider, sortedKeys[i])
			require.NoError(t, err)

			digest1, err := d1.DigestPrefix(d1.Levels())
			require.NoError(t, err)

			d2, err := digesterBuilder.Digest(hashInputProvider, sortedKeys[j])
			require.NoError(t, err)

			digest2, err := d2.DigestPrefix(d2.Levels())
			require.NoError(t, err)

			for z := 0; z < len(digest1); z++ {
				if digest1[z] != digest2[z] {
					return digest1[z] < digest2[z] // sort by hkey
				}
			}
			return i < j // sort by insertion order with hash collision
		})

		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		for _, k := range keys {
			v, ok := uniqueKeyValues[k]
			require.True(t, ok)

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		// Iterate key value pairs
		i := uint64(0)
		err = m.Iterate(func(k Value, v Value) (resume bool, err error) {
			require.Equal(t, sortedKeys[i], k)
			require.Equal(t, uniqueKeyValues[k], v)

			i++

			return true, nil
		})
		require.NoError(t, err)
		require.Equal(t, i, uint64(mapSize))

		// Iterate keys
		i = uint64(0)
		err = m.IterateKeys(func(k Value) (resume bool, err error) {
			require.Equal(t, sortedKeys[i], k)
			i++
			return true, nil
		})
		require.NoError(t, err)
		require.Equal(t, i, uint64(mapSize))

		// Iterate values
		i = uint64(0)
		err = m.IterateValues(func(v Value) (resume bool, err error) {
			require.Equal(t, uniqueKeyValues[sortedKeys[i]], v)
			i++
			return true, nil
		})
		require.NoError(t, err)
		require.Equal(t, i, uint64(mapSize))

		require.Equal(t, typeInfo, m.Type())
	})
}

func testMapDeterministicHashCollision(t *testing.T, maxDigestLevel int) {

	const mapSize = 2 * 1024

	// mockDigestCount is the number of unique set of digests.
	// Each set has maxDigestLevel of digest.
	const mockDigestCount = 8

	typeInfo := testTypeInfo{42}

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	digesterBuilder := &mockDigesterBuilder{}

	// Generate mockDigestCount*maxDigestLevel number of unique digest
	digests := make([]Digest, 0, mockDigestCount*maxDigestLevel)
	uniqueDigest := make(map[Digest]bool)
	for len(uniqueDigest) < mockDigestCount*maxDigestLevel {
		d := Digest(uint64(rand.Intn(256)))
		if !uniqueDigest[d] {
			uniqueDigest[d] = true
			digests = append(digests, d)
		}
	}

	storage := newTestInMemoryStorage(t)

	uniqueKeyValues := make(map[Value]Value, mapSize)
	uniqueKeys := make(map[string]bool)
	for i := uint64(0); i < mapSize; i++ {
		for {
			s := randStr(16)
			if !uniqueKeys[s] {
				uniqueKeys[s] = true

				k := NewStringValue(s)
				uniqueKeyValues[k] = NewStringValue(randStr(16))

				index := (i % mockDigestCount)
				startIndex := int(index) * maxDigestLevel
				endIndex := int(index)*maxDigestLevel + maxDigestLevel

				digests := digests[startIndex:endIndex]

				digesterBuilder.On("Digest", k).Return(mockDigester{digests})

				break
			}
		}
	}

	m, err := NewMap(storage, address, digesterBuilder, typeInfo)
	require.NoError(t, err)

	for k, v := range uniqueKeyValues {
		existingStorable, err := m.Set(compare, hashInputProvider, k, v)
		require.NoError(t, err)
		require.Nil(t, existingStorable)
	}

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	for k, v := range uniqueKeyValues {
		strv := k.(StringValue)
		require.NotNil(t, strv)

		s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
		require.NoError(t, err)

		e, err := s.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, e)
	}

	require.Equal(t, typeInfo, m.Type())

	stats, _ := GetMapStats(m)
	require.Equal(t, stats.DataSlabCount+stats.MetaDataSlabCount+stats.CollisionDataSlabCount, uint64(m.Storage.Count()))
	require.Equal(t, uint64(mockDigestCount), stats.CollisionDataSlabCount)

	for k, v := range uniqueKeyValues {
		strv := k.(StringValue)
		require.NotNil(t, strv)

		removedKeyStorable, removedValueStorable, err := m.Remove(compare, hashInputProvider, NewStringValue(strv.str))
		require.NoError(t, err)

		removedKey, err := removedKeyStorable.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, k, removedKey)

		removedValue, err := removedValueStorable.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, removedValue)
	}

	require.Equal(t, uint64(0), m.Count())

	require.Equal(t, typeInfo, m.Type())

	require.Equal(t, uint64(1), uint64(m.Storage.Count()))

	stats, _ = GetMapStats(m)
	require.Equal(t, uint64(1), stats.DataSlabCount)
	require.Equal(t, uint64(0), stats.MetaDataSlabCount)
	require.Equal(t, uint64(0), stats.CollisionDataSlabCount)
}

func testMapRandomHashCollision(t *testing.T, maxDigestLevel int) {

	const mapSize = 2 * 1024

	typeInfo := testTypeInfo{42}

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	digesterBuilder := &mockDigesterBuilder{}

	storage := newTestInMemoryStorage(t)

	uniqueKeyValues := make(map[Value]Value, mapSize)
	uniqueKeys := make(map[string]bool)
	for i := uint64(0); i < mapSize; i++ {
		for {
			s := randStr(rand.Intn(16))

			if !uniqueKeys[s] {
				uniqueKeys[s] = true

				k := NewStringValue(s)
				uniqueKeyValues[k] = NewStringValue(randStr(16))

				var digests []Digest
				for i := 0; i < maxDigestLevel; i++ {
					digests = append(digests, Digest(rand.Intn(256)))
				}

				digesterBuilder.On("Digest", k).Return(mockDigester{digests})

				break
			}
		}
	}

	m, err := NewMap(storage, address, digesterBuilder, typeInfo)
	require.NoError(t, err)

	for k, v := range uniqueKeyValues {
		existingStorable, err := m.Set(compare, hashInputProvider, k, v)
		require.NoError(t, err)
		require.Nil(t, existingStorable)
	}

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	for k, v := range uniqueKeyValues {
		strv := k.(StringValue)
		require.NotNil(t, strv)

		s, err := m.Get(compare, hashInputProvider, NewStringValue(strv.str))
		require.NoError(t, err)

		e, err := s.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, e)
	}

	require.Equal(t, typeInfo, m.Type())

	stats, _ := GetMapStats(m)
	require.Equal(t, stats.DataSlabCount+stats.MetaDataSlabCount+stats.CollisionDataSlabCount, uint64(m.Storage.Count()))

	for k, v := range uniqueKeyValues {
		strv := k.(StringValue)
		require.NotNil(t, strv)

		removedKeyStorable, removedValueStorable, err := m.Remove(compare, hashInputProvider, NewStringValue(strv.str))
		require.NoError(t, err)

		removedKey, err := removedKeyStorable.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, k, removedKey)

		removedValue, err := removedValueStorable.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, removedValue)
	}

	require.Equal(t, uint64(0), m.Count())

	require.Equal(t, typeInfo, m.Type())

	require.Equal(t, uint64(1), uint64(m.Storage.Count()))

	stats, _ = GetMapStats(m)
	require.Equal(t, uint64(1), stats.DataSlabCount)
	require.Equal(t, uint64(0), stats.MetaDataSlabCount)
	require.Equal(t, uint64(0), stats.CollisionDataSlabCount)
}

func TestMapHashCollision(t *testing.T) {

	SetThreshold(512)
	defer func() {
		SetThreshold(1024)
	}()

	const maxDigestLevel = 4

	for hashLevel := 1; hashLevel <= maxDigestLevel; hashLevel++ {
		name := fmt.Sprintf("deterministic max hash level %d", hashLevel)
		t.Run(name, func(t *testing.T) {
			testMapDeterministicHashCollision(t, hashLevel)
		})
	}

	for hashLevel := 1; hashLevel <= maxDigestLevel; hashLevel++ {
		name := fmt.Sprintf("random max hash level %d", hashLevel)
		t.Run(name, func(t *testing.T) {
			testMapRandomHashCollision(t, hashLevel)
		})
	}
}

func TestMapLargeElement(t *testing.T) {

	SetThreshold(512)
	defer func() {
		SetThreshold(1024)
	}()

	typeInfo := testTypeInfo{42}

	const mapSize = 2 * 1024

	const keySize = 512
	const valueSize = 512

	strs := make(map[string]string, mapSize)
	for i := uint64(0); i < mapSize; i++ {
		k := randStr(keySize)
		v := randStr(valueSize)
		strs[k] = v
	}

	storage := newTestInMemoryStorage(t)

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
	require.NoError(t, err)

	for k, v := range strs {
		existingStorable, err := m.Set(compare, hashInputProvider, NewStringValue(k), NewStringValue(v))
		require.NoError(t, err)
		require.Nil(t, existingStorable)
	}

	for k, v := range strs {
		s, err := m.Get(compare, hashInputProvider, NewStringValue(k))
		require.NoError(t, err)

		e, err := s.StoredValue(storage)
		require.NoError(t, err)

		sv, ok := e.(StringValue)
		require.True(t, ok)
		require.Equal(t, v, sv.str)
	}

	require.Equal(t, typeInfo, m.Type())
	require.Equal(t, uint64(mapSize), m.Count())

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	stats, _ := GetMapStats(m)
	require.Equal(t, stats.DataSlabCount+stats.MetaDataSlabCount+mapSize*2, uint64(m.Storage.Count()))
}

func TestMapRandomSetRemoveMixedTypes(t *testing.T) {

	const (
		SetAction = iota
		RemoveAction
		MaxAction
	)

	const (
		Uint8Type = iota
		Uint16Type
		Uint32Type
		Uint64Type
		StringType
		MaxType
	)

	SetThreshold(256)
	defer func() {
		SetThreshold(1024)
	}()

	const actionCount = 2 * 1024

	const digestMaxValue = 256

	const digestMaxLevels = 4

	const stringMaxSize = 512

	typeInfo := testTypeInfo{42}

	storage := newTestInMemoryStorage(t)

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	digesterBuilder := &mockDigesterBuilder{}

	m, err := NewMap(storage, address, digesterBuilder, typeInfo)
	require.NoError(t, err)

	keyValues := make(map[Value]Value)
	var keys []Value

	for i := uint64(0); i < actionCount; i++ {

		switch rand.Intn(MaxAction) {

		case SetAction:

			var k Value

			switch rand.Intn(MaxType) {
			case Uint8Type:
				n := rand.Intn(math.MaxUint8 + 1)
				k = Uint8Value(n)
			case Uint16Type:
				n := rand.Intn(math.MaxUint16 + 1)
				k = Uint16Value(n)
			case Uint32Type:
				k = Uint32Value(rand.Uint32())
			case Uint64Type:
				k = Uint64Value(rand.Uint64())
			case StringType:
				k = NewStringValue(randStr(rand.Intn(stringMaxSize)))
			}

			var v Value

			switch rand.Intn(MaxType) {
			case Uint8Type:
				n := rand.Intn(math.MaxUint8 + 1)
				v = Uint8Value(n)
			case Uint16Type:
				n := rand.Intn(math.MaxUint16 + 1)
				v = Uint16Value(n)
			case Uint32Type:
				v = Uint32Value(rand.Uint32())
			case Uint64Type:
				v = Uint64Value(rand.Uint64())
			case StringType:
				v = NewStringValue(randStr(rand.Intn(stringMaxSize)))
			}

			var digests []Digest
			for i := 0; i < digestMaxLevels; i++ {
				digests = append(digests, Digest(rand.Intn(digestMaxValue)))
			}

			digesterBuilder.On("Digest", k).Return(mockDigester{digests})

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)

			if oldv, ok := keyValues[k]; ok {
				require.NotNil(t, existingStorable)

				existingValue, err := existingStorable.StoredValue(storage)
				require.NoError(t, err)
				require.Equal(t, oldv, existingValue)
			} else {
				require.Nil(t, existingStorable)
			}

			if existingStorable == nil {
				keys = append(keys, k)
			}

			keyValues[k] = v

		case RemoveAction:
			if len(keys) > 0 {
				ki := rand.Intn(len(keys))
				k := keys[ki]

				removedKeyStorable, removedValueStorable, err := m.Remove(compare, hashInputProvider, k)
				require.NoError(t, err)

				removedKey, err := removedKeyStorable.StoredValue(storage)
				require.NoError(t, err)
				require.Equal(t, k, removedKey)

				removedValue, err := removedValueStorable.StoredValue(storage)
				require.NoError(t, err)
				require.Equal(t, keyValues[k], removedValue)

				delete(keyValues, k)
				copy(keys[ki:], keys[ki+1:])
				keys = keys[:len(keys)-1]
			}
		}

		require.Equal(t, m.Count(), uint64(len(keys)))
		require.Equal(t, typeInfo, m.Type())
	}

	for k, v := range keyValues {
		s, err := m.Get(compare, hashInputProvider, k)
		require.NoError(t, err)

		e, err := s.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, e)
	}

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

}

func TestMapEncodeDecode(t *testing.T) {

	encMode, err := cbor.EncOptions{}.EncMode()
	require.NoError(t, err)

	decMode, err := cbor.DecOptions{}.DecMode()
	require.NoError(t, err)

	typeInfo := testTypeInfo{42}

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	t.Run("empty", func(t *testing.T) {

		storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		// Create map
		m, err := NewMap(storage, address, NewDefaultDigesterBuilder(), typeInfo)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}

		expected := map[StorageID][]byte{
			id1: {
				// extra data
				// version
				0x00,
				// flag: root + map data
				0x88,
				// extra data (CBOR encoded array of 3 elements)
				0x83,
				// type info
				0x18, 0x2a,
				// count: 0
				0x00,
				// seed
				0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

				// version
				0x00,
				// flag: root + map data
				0x88,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 1)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// elements (array of 0 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		}

		// Verify encoded data
		stored, err := storage.Encode()
		require.NoError(t, err)
		require.Equal(t, expected[id1], stored[id1])

		// Decode data to new storage
		storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		err = storage2.Load(stored)
		require.NoError(t, err)

		// Test new map from storage2
		decodedMap, err := NewMapWithRootID(storage2, id1, NewDefaultDigesterBuilder())
		require.NoError(t, err)

		require.Equal(t, uint64(0), decodedMap.Count())
		require.Equal(t, typeInfo, decodedMap.Type())

		storable, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(0))
		require.Error(t, err, KeyNotFoundError{})
		require.Nil(t, storable)
	})

	t.Run("no pointer no collision", func(t *testing.T) {

		SetThreshold(100)
		defer func() {
			SetThreshold(1024)
		}()

		// Create and populate map in memory
		storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		digesterBuilder := &mockDigesterBuilder{}

		// Create map
		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		const mapSize = 10
		for i := uint64(0); i < mapSize; i++ {
			k := Uint64Value(i)
			v := Uint64Value(i * 2)

			digests := []Digest{Digest(i), Digest(i * 2)}
			digesterBuilder.On("Digest", k).Return(mockDigester{d: digests})

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		require.Equal(t, uint64(mapSize), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}
		id2 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 2}}
		id3 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 3}}

		// Expected serialized slab data with storage id
		expected := map[StorageID][]byte{

			// metadata slab
			id1: {
				// extra data
				// version
				0x00,
				// flag: root + map meta
				0x89,
				// extra data (CBOR encoded array of 3 elements)
				0x83,
				// type info: "map"
				//0x63, 0x6d, 0x61, 0x70,
				0x18, 0x2A,
				// count: 10
				0x0a,
				// seed
				0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

				// version
				0x00,
				// flag: root + meta
				0x89,
				// child header count
				0x00, 0x02,
				// child header 1 (storage id, first key, size)
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x71,
				// child header 2
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				0x00, 0x00, 0x00, 0x71,
			},

			// data slab
			id2: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 5)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x28,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				// hkey: 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// hkey: 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// hkey: 4
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04,

				// elements (array of 5 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// element: [uint64(0), uint64(0)]
				0x82, 0xd8, 0xa4, 0x00, 0xd8, 0xa4, 0x00,
				// element: [uint64(1), uint64(2)]
				0x82, 0xd8, 0xa4, 0x01, 0xd8, 0xa4, 0x02,
				// element: [uint64(2), uint64(4)]
				0x82, 0xd8, 0xa4, 0x02, 0xd8, 0xa4, 0x04,
				// element: [uint64(3), uint64(6)]
				0x82, 0xd8, 0xa4, 0x03, 0xd8, 0xa4, 0x06,
				// element: [uint64(4), uint64(8)]
				0x82, 0xd8, 0xa4, 0x04, 0xd8, 0xa4, 0x08,
			},

			// data slab
			id3: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 5)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x28,
				// hkey: 5
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// hkey: 6
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06,
				// hkey: 7
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07,
				// hkey: 8
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 9
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,

				// elements (array of 5 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// element: [uint64(5), uint64(10)]
				0x82, 0xd8, 0xa4, 0x05, 0xd8, 0xa4, 0x0a,
				// element: [uint64(6), uint64(12)]
				0x82, 0xd8, 0xa4, 0x06, 0xd8, 0xa4, 0x0c,
				// element: [uint64(7), uint64(14)]
				0x82, 0xd8, 0xa4, 0x07, 0xd8, 0xa4, 0x0e,
				// element: [uint64(8), uint64(16)]
				0x82, 0xd8, 0xa4, 0x08, 0xd8, 0xa4, 0x10,
				// element: [uint64(9), uint64(18)]
				0x82, 0xd8, 0xa4, 0x09, 0xd8, 0xa4, 0x12,
			},
		}

		// Verify encoded data
		stored, err := storage.Encode()
		require.NoError(t, err)

		require.Equal(t, expected[id1], stored[id1])
		require.Equal(t, expected[id2], stored[id2])
		require.Equal(t, expected[id3], stored[id3])

		// Verify slab size in header is correct.
		meta, ok := m.root.(*MapMetaDataSlab)
		require.True(t, ok)
		require.Equal(t, 2, len(meta.childrenHeaders))
		require.Equal(t, uint32(len(stored[id2])), meta.childrenHeaders[0].size)
		require.Equal(t, uint32(len(stored[id3])), meta.childrenHeaders[1].size)

		// Decode data to new storage
		storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		err = storage2.Load(stored)
		require.NoError(t, err)

		// Test new map from storage2
		decodedMap, err := NewMapWithRootID(storage2, id1, digesterBuilder)
		require.NoError(t, err)

		require.Equal(t, uint64(mapSize), decodedMap.Count())
		require.Equal(t, typeInfo, decodedMap.Type())

		for i := uint64(0); i < mapSize; i++ {
			s, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(i))
			require.NoError(t, err)

			v, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, Uint64Value(i*2), v)
		}
	})

	t.Run("has pointer no collision", func(t *testing.T) {

		SetThreshold(100)
		defer func() {
			SetThreshold(1024)
		}()

		// Create and populate map in memory
		storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		digesterBuilder := &mockDigesterBuilder{}

		// Create map
		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		const mapSize = 10
		for i := uint64(0); i < mapSize; i++ {
			k := Uint64Value(i)
			v := Uint64Value(i * 2)

			digests := []Digest{Digest(i), Digest(i * 2)}
			digesterBuilder.On("Digest", k).Return(mockDigester{d: digests})

			if i == mapSize-1 {
				// Create nested array
				typeInfo2 := testTypeInfo{43}

				array, err := NewArray(storage, address, typeInfo2)
				require.NoError(t, err)

				err = array.Append(Uint64Value(0))
				require.NoError(t, err)

				// Insert array to map
				existingStorable, err := m.Set(compare, hashInputProvider, k, array)
				require.NoError(t, err)
				require.Nil(t, existingStorable)
			} else {
				existingStorable, err := m.Set(compare, hashInputProvider, k, v)
				require.NoError(t, err)
				require.Nil(t, existingStorable)
			}
		}

		require.Equal(t, uint64(mapSize), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}
		id2 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 2}}
		id3 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 3}}
		id4 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 4}}

		// Expected serialized slab data with storage id
		expected := map[StorageID][]byte{

			// metadata slab
			id1: {
				// extra data
				// version
				0x00,
				// flag: root + map meta
				0x89,
				// extra data (CBOR encoded array of 3 elements)
				0x83,
				// type info: "map"
				//0x63, 0x6d, 0x61, 0x70,
				0x18, 0x2A,
				// count: 10
				0x0a,
				// seed
				0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

				// version
				0x00,
				// flag: root + map meta
				0x89,
				// child header count
				0x00, 0x02,
				// child header 1 (storage id, first key, size)
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x71,
				// child header 2
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				0x00, 0x00, 0x00, 0x81,
			},
			// data slab
			id2: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 5)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x28,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				// hkey: 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// hkey: 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// hkey: 4
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04,

				// elements (array of 5 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// element: [uint64(0), uint64(0)]
				0x82, 0xd8, 0xa4, 0x00, 0xd8, 0xa4, 0x00,
				// element: [uint64(1), uint64(2)]
				0x82, 0xd8, 0xa4, 0x01, 0xd8, 0xa4, 0x02,
				// element: [uint64(2), uint64(4)]
				0x82, 0xd8, 0xa4, 0x02, 0xd8, 0xa4, 0x04,
				// element: [uint64(3), uint64(6)]
				0x82, 0xd8, 0xa4, 0x03, 0xd8, 0xa4, 0x06,
				// element: [uint64(4), uint64(8)]
				0x82, 0xd8, 0xa4, 0x04, 0xd8, 0xa4, 0x08,
			},
			// data slab
			id3: {
				// version
				0x00,
				// flag: has pointer + map data
				0x48,
				// next storage id
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 6)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x28,
				// hkey: 5
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// hkey: 6
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06,
				// hkey: 7
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07,
				// hkey: 8
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 9
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,

				// elements (array of 5 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// element: [uint64(5), uint64(10)]
				0x82, 0xd8, 0xa4, 0x05, 0xd8, 0xa4, 0x0a,
				// element: [uint64(6), uint64(12)]
				0x82, 0xd8, 0xa4, 0x06, 0xd8, 0xa4, 0x0c,
				// element: [uint64(7), uint64(14)]
				0x82, 0xd8, 0xa4, 0x07, 0xd8, 0xa4, 0x0e,
				// element: [uint64(8), uint64(16)]
				0x82, 0xd8, 0xa4, 0x08, 0xd8, 0xa4, 0x10,
				// element: [uint64(9), StorageID(1,2,3,4,5,6,7,8,0,0,0,0,0,0,0,4)]
				0x82, 0xd8, 0xa4, 0x09, 0xd8, 0xff, 0x50, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04,
			},
			// array data slab
			id4: {
				// extra data
				// version
				0x00,
				// flag: root + array data
				0x80,
				// extra data (CBOR encoded array of 1 elements)
				0x81,
				// type info
				0x18, 0x2b,

				// version
				0x00,
				// flag: root + array data
				0x80,
				// CBOR encoded array head (fixed size 3 byte)
				0x99, 0x00, 0x01,
				// CBOR encoded array elements
				0xd8, 0xa4, 0x00,
			},
		}

		stored, err := storage.Encode()
		require.NoError(t, err)

		require.Equal(t, 4, len(stored))
		require.Equal(t, expected[id1], stored[id1])
		require.Equal(t, expected[id2], stored[id2])
		require.Equal(t, expected[id3], stored[id3])
		require.Equal(t, expected[id4], stored[id4])

		// Verify slab size in header is correct.
		meta, ok := m.root.(*MapMetaDataSlab)
		require.True(t, ok)
		require.Equal(t, 2, len(meta.childrenHeaders))
		require.Equal(t, uint32(len(stored[id2])), meta.childrenHeaders[0].size)
		require.Equal(t, uint32(len(stored[id3])), meta.childrenHeaders[1].size)

		// Decode data to new storage
		storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		err = storage2.Load(stored)
		require.NoError(t, err)

		// Test new map from storage2
		decodedMap, err := NewMapWithRootID(storage2, id1, digesterBuilder)
		require.NoError(t, err)

		require.Equal(t, uint64(mapSize), decodedMap.Count())
		require.Equal(t, typeInfo, decodedMap.Type())

		for i := uint64(0); i < mapSize; i++ {

			if i == mapSize-1 {
				// Get nested array
				storable, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(i))
				require.NoError(t, err)

				v, err := storable.StoredValue(storage)
				require.NoError(t, err)

				a, ok := v.(*Array)
				require.True(t, ok)

				require.Equal(t, uint64(1), a.Count())
				storable, err = a.Get(0)
				require.NoError(t, err)

				s, err := storable.StoredValue(storage)
				require.NoError(t, err)
				require.Equal(t, Uint64Value(0), s)
			} else {
				s, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(i))
				require.NoError(t, err)

				v, err := s.StoredValue(storage)
				require.NoError(t, err)
				require.Equal(t, Uint64Value(i*2), v)
			}
		}
	})

	t.Run("inline collision 1 level", func(t *testing.T) {

		SetThreshold(150)
		defer func() {
			SetThreshold(1024)
		}()

		// Create and populate map in memory
		storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		digesterBuilder := &mockDigesterBuilder{}

		// Create map
		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		const mapSize = 10
		for i := uint64(0); i < mapSize; i++ {
			k := Uint64Value(i)
			v := Uint64Value(i * 2)

			digests := []Digest{Digest(i % 4), Digest(i)}
			digesterBuilder.On("Digest", k).Return(mockDigester{d: digests})

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		require.Equal(t, uint64(mapSize), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}
		id2 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 2}}
		id3 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 3}}

		// Expected serialized slab data with storage id
		expected := map[StorageID][]byte{

			// map metadata slab
			id1: {
				// extra data
				// version
				0x00,
				// flag: root + meta
				0x89,
				// extra data (CBOR encoded array of 3 elements)
				0x83,
				// type info: "map"
				//0x63, 0x6d, 0x61, 0x70,
				0x18, 0x2A,
				// count: 10
				0x0a,
				// seed
				0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

				// version
				0x00,
				// flag: root + map meta
				0x89,
				// child header count
				0x00, 0x02,
				// child header 1 (storage id, first key, size)
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0xbc,
				// child header 2
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x9e,
			},
			// map data slab
			id2: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// elements (array of 2 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,

				// inline collision group corresponding to hkey 0
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 3)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x18,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// hkey: 4
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04,
				// hkey: 8
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,

				// elements (array of 3 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// element: [uint64(0), uint64(0)]
				0x82, 0xd8, 0xa4, 0x00, 0xd8, 0xa4, 0x00,
				// element: [uint64(4), uint64(8)]
				0x82, 0xd8, 0xa4, 0x04, 0xd8, 0xa4, 0x08,
				// element: [uint64(8), uint64(16)]
				0x82, 0xd8, 0xa4, 0x08, 0xd8, 0xa4, 0x10,

				// inline collision group corresponding to hkey 1
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 3)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x18,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				// hkey: 5
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// hkey: 9
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,

				// elements (array of 3 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// element: [uint64(1), uint64(2)]
				0x82, 0xd8, 0xa4, 0x01, 0xd8, 0xa4, 0x02,
				// element: [uint64(5), uint64(10)]
				0x82, 0xd8, 0xa4, 0x05, 0xd8, 0xa4, 0x0a,
				// element: [uint64(9), uint64(18)]
				0x82, 0xd8, 0xa4, 0x09, 0xd8, 0xa4, 0x12,
			},

			// map data slab
			id3: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// hkey: 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,

				// elements (array of 2 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,

				// inline collision group corresponding to hkey 2
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// hkey: 6
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06,

				// elements (array of 2 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// element: [uint64(2), uint64(4)]
				0x82, 0xd8, 0xa4, 0x02, 0xd8, 0xa4, 0x04,
				// element: [uint64(6), uint64(12)]
				0x82, 0xd8, 0xa4, 0x06, 0xd8, 0xa4, 0x0c,

				// inline collision group corresponding to hkey 3
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// hkey: 7
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07,

				// elements (array of 2 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// element: [uint64(3), uint64(6)]
				0x82, 0xd8, 0xa4, 0x03, 0xd8, 0xa4, 0x06,
				// element: [uint64(7), uint64(14)]
				0x82, 0xd8, 0xa4, 0x07, 0xd8, 0xa4, 0x0e,
			},
		}

		stored, err := storage.Encode()
		require.NoError(t, err)

		require.Equal(t, expected[id1], stored[id1])
		require.Equal(t, expected[id2], stored[id2])
		require.Equal(t, expected[id3], stored[id3])

		// Verify slab size in header is correct.
		meta, ok := m.root.(*MapMetaDataSlab)
		require.True(t, ok)
		require.Equal(t, 2, len(meta.childrenHeaders))
		require.Equal(t, uint32(len(stored[id2])), meta.childrenHeaders[0].size)
		require.Equal(t, uint32(len(stored[id3])), meta.childrenHeaders[1].size)

		// Decode data to new storage
		storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		err = storage2.Load(stored)
		require.NoError(t, err)

		// Test new map from storage2
		decodedMap, err := NewMapWithRootID(storage2, id1, digesterBuilder)
		require.NoError(t, err)

		require.Equal(t, uint64(mapSize), decodedMap.Count())
		require.Equal(t, typeInfo, decodedMap.Type())

		for i := uint64(0); i < mapSize; i++ {
			s, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(i))
			require.NoError(t, err)

			v, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, Uint64Value(i*2), v)
		}
	})

	t.Run("inline collision 2 levels", func(t *testing.T) {

		SetThreshold(150)
		defer func() {
			SetThreshold(1024)
		}()

		// Create and populate map in memory
		storage := NewBasicSlabStorage(encMode, decMode, nil, nil)
		storage.DecodeStorable = decodeStorable

		digesterBuilder := &mockDigesterBuilder{}

		// Create map
		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		const mapSize = 10
		for i := uint64(0); i < mapSize; i++ {
			k := Uint64Value(i)
			v := Uint64Value(i * 2)

			digests := []Digest{Digest(i % 4), Digest(i % 2)}
			digesterBuilder.On("Digest", k).Return(mockDigester{d: digests})

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		require.Equal(t, uint64(mapSize), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}
		id2 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 2}}
		id3 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 3}}

		// Expected serialized slab data with storage id
		expected := map[StorageID][]byte{

			// map metadata slab
			id1: {
				// extra data
				// version
				0x00,
				// flag: root + meta
				0x89,
				// extra data (CBOR encoded array of 3 elements)
				0x83,
				// type info: "map"
				//0x63, 0x6d, 0x61, 0x70,
				0x18, 0x2A,
				// count: 10
				0x0a,
				// seed
				0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

				// version
				0x00,
				// flag: root + map meta
				0x89,
				// child header count
				0x00, 0x02,
				// child header 1 (storage id, first key, size)
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0xb8,
				// child header 2
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0xaa,
			},

			// map data slab
			id2: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// elements (array of 2 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,

				// inline collision group corresponding to hkey 0
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level 1
				0x01,

				// hkeys (byte string of length 8 * 1)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// elements (array of 1 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// inline collision group corresponding to hkey [0, 0]
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 2
				0x02,

				// hkeys (empty byte string)
				0x40,

				// elements (array of 3 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// element: [uint64(0), uint64(0)]
				0x82, 0xd8, 0xa4, 0x00, 0xd8, 0xa4, 0x00,
				// element: [uint64(4), uint64(8)]
				0x82, 0xd8, 0xa4, 0x04, 0xd8, 0xa4, 0x08,
				// element: [uint64(8), uint64(16)]
				0x82, 0xd8, 0xa4, 0x08, 0xd8, 0xa4, 0x10,

				// inline collision group corresponding to hkey 1
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 1)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// elements (array of 1 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// inline collision group corresponding to hkey [1, 1]
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 2
				0x02,

				// hkeys (empty byte string)
				0x40,

				// elements (array of 3 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// element: [uint64(1), uint64(2)]
				0x82, 0xd8, 0xa4, 0x01, 0xd8, 0xa4, 0x02,
				// element: [uint64(5), uint64(10)]
				0x82, 0xd8, 0xa4, 0x05, 0xd8, 0xa4, 0x0a,
				// element: [uint64(9), uint64(18)]
				0x82, 0xd8, 0xa4, 0x09, 0xd8, 0xa4, 0x12,
			},

			// map data slab
			id3: {
				// version
				0x00,
				// flag: map data
				0x08,
				// next storage id
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// hkey: 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,

				// elements (array of 2 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,

				// inline collision group corresponding to hkey 2
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 1)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// elements (array of 1 element)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// inline collision group corresponding to hkey [2, 0]
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 2
				0x02,

				// hkeys (empty byte string)
				0x40,

				// elements (array of 2 element)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// element: [uint64(2), uint64(4)]
				0x82, 0xd8, 0xa4, 0x02, 0xd8, 0xa4, 0x04,
				// element: [uint64(6), uint64(12)]
				0x82, 0xd8, 0xa4, 0x06, 0xd8, 0xa4, 0x0c,

				// inline collision group corresponding to hkey 3
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 1)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// elements (array of 1 element)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// inline collision group corresponding to hkey [3, 1]
				// (tag number CBORTagInlineCollisionGroup)
				0xd8, 0xfd,
				// (tag content: array of 3 elements)
				0x83,

				// level: 2
				0x02,

				// hkeys (empty byte string)
				0x40,

				// elements (array of 2 element)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// element: [uint64(3), uint64(6)]
				0x82, 0xd8, 0xa4, 0x03, 0xd8, 0xa4, 0x06,
				// element: [uint64(7), uint64(14)]
				0x82, 0xd8, 0xa4, 0x07, 0xd8, 0xa4, 0x0e,
			},
		}

		stored, err := storage.Encode()
		require.NoError(t, err)

		require.Equal(t, expected[id1], stored[id1])
		require.Equal(t, expected[id2], stored[id2])
		require.Equal(t, expected[id3], stored[id3])

		// Verify slab size in header is correct.
		meta, ok := m.root.(*MapMetaDataSlab)
		require.True(t, ok)
		require.Equal(t, 2, len(meta.childrenHeaders))
		require.Equal(t, uint32(len(stored[id2])), meta.childrenHeaders[0].size)
		require.Equal(t, uint32(len(stored[id3])), meta.childrenHeaders[1].size)

		// Decode data to new storage
		storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		err = storage2.Load(stored)
		require.NoError(t, err)

		// Test new map from storage2
		decodedMap, err := NewMapWithRootID(storage2, id1, digesterBuilder)
		require.NoError(t, err)

		require.Equal(t, uint64(mapSize), decodedMap.Count())
		require.Equal(t, typeInfo, decodedMap.Type())

		for i := uint64(0); i < mapSize; i++ {
			s, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(i))
			require.NoError(t, err)

			v, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, Uint64Value(i*2), v)
		}
	})

	t.Run("external collision", func(t *testing.T) {

		SetThreshold(100)
		defer func() {
			SetThreshold(1024)
		}()

		// Create and populate map in memory
		storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		digesterBuilder := &mockDigesterBuilder{}

		// Create map
		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		const mapSize = 20
		for i := uint64(0); i < mapSize; i++ {
			k := Uint64Value(i)
			v := Uint64Value(i * 2)

			digests := []Digest{Digest(i % 2), Digest(i)}
			digesterBuilder.On("Digest", k).Return(mockDigester{d: digests})

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		require.Equal(t, uint64(mapSize), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}
		id2 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 2}}
		id3 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 3}}

		// Expected serialized slab data with storage id
		expected := map[StorageID][]byte{

			// map data slab
			id1: {
				// extra data
				// version
				0x00,
				// flag: root + has pointer + map data
				0xc8,
				// extra data (CBOR encoded array of 3 elements)
				0x83,
				// type info: "map"
				//0x63, 0x6d, 0x61, 0x70,
				0x18, 0x2A,
				// count: 10
				0x14,
				// seed
				0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

				// version
				0x00,
				// flag: root + has pointer + map data
				0xc8,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 0
				0x00,

				// hkeys (byte string of length 8 * 2)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,

				// elements (array of 2 elements)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,

				// external collision group corresponding to hkey 0
				// (tag number CBORTagExternalCollisionGroup)
				0xd8, 0xfe,
				// (tag content: storage id)
				0xd8, 0xff, 0x50,
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,

				// external collision group corresponding to hkey 1
				// (tag number CBORTagExternalCollisionGroup)
				0xd8, 0xfe,
				// (tag content: storage id)
				0xd8, 0xff, 0x50,
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
			},

			// external collision group
			id2: {
				// version
				0x00,
				// flag: any size + collision group
				0x2b,
				// next storage id
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 10)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x50,
				// hkey: 0
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// hkey: 2
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
				// hkey: 4
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04,
				// hkey: 6
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06,
				// hkey: 8
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
				// hkey: 10
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0a,
				// hkey: 12
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0c,
				// hkey: 14
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e,
				// hkey: 16
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
				// hkey: 18
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12,

				// elements (array of 10 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0a,
				// element: [uint64(0), uint64(0)]
				0x82, 0xd8, 0xa4, 0x00, 0xd8, 0xa4, 0x00,
				// element: [uint64(2), uint64(4)]
				0x82, 0xd8, 0xa4, 0x02, 0xd8, 0xa4, 0x04,
				// element: [uint64(4), uint64(8)]
				0x82, 0xd8, 0xa4, 0x04, 0xd8, 0xa4, 0x08,
				// element: [uint64(6), uint64(12)]
				0x82, 0xd8, 0xa4, 0x06, 0xd8, 0xa4, 0x0c,
				// element: [uint64(8), uint64(16)]
				0x82, 0xd8, 0xa4, 0x08, 0xd8, 0xa4, 0x10,
				// element: [uint64(10), uint64(20)]
				0x82, 0xd8, 0xa4, 0x0a, 0xd8, 0xa4, 0x14,
				// element: [uint64(12), uint64(24)]
				0x82, 0xd8, 0xa4, 0x0c, 0xd8, 0xa4, 0x18, 0x18,
				// element: [uint64(14), uint64(28)]
				0x82, 0xd8, 0xa4, 0x0e, 0xd8, 0xa4, 0x18, 0x1c,
				// element: [uint64(16), uint64(32)]
				0x82, 0xd8, 0xa4, 0x10, 0xd8, 0xa4, 0x18, 0x20,
				// element: [uint64(18), uint64(36)]
				0x82, 0xd8, 0xa4, 0x12, 0xd8, 0xa4, 0x18, 0x24,
			},

			// external collision group
			id3: {
				// version
				0x00,
				// flag: any size + collision group
				0x2b,
				// next storage id
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

				// the following encoded data is valid CBOR

				// elements (array of 3 elements)
				0x83,

				// level: 1
				0x01,

				// hkeys (byte string of length 8 * 10)
				0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x50,
				// hkey: 1
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				// hkey: 3
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03,
				// hkey: 5
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				// hkey: 7
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07,
				// hkey: 9
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,
				// hkey: 11
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0b,
				// hkey: 13
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0d,
				// hkey: 15
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0f,
				// hkey: 17
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x11,
				// hkey: 19
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x13,

				// elements (array of 10 elements)
				// each element is encoded as CBOR array of 2 elements (key, value)
				0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0a,
				// element: [uint64(1), uint64(2)]
				0x82, 0xd8, 0xa4, 0x01, 0xd8, 0xa4, 0x02,
				// element: [uint64(3), uint64(6)]
				0x82, 0xd8, 0xa4, 0x03, 0xd8, 0xa4, 0x06,
				// element: [uint64(5), uint64(10)]
				0x82, 0xd8, 0xa4, 0x05, 0xd8, 0xa4, 0x0a,
				// element: [uint64(7), uint64(14)]
				0x82, 0xd8, 0xa4, 0x07, 0xd8, 0xa4, 0x0e,
				// element: [uint64(9), uint64(18)]
				0x82, 0xd8, 0xa4, 0x09, 0xd8, 0xa4, 0x12,
				// element: [uint64(11), uint64(22))]
				0x82, 0xd8, 0xa4, 0x0b, 0xd8, 0xa4, 0x16,
				// element: [uint64(13), uint64(26)]
				0x82, 0xd8, 0xa4, 0x0d, 0xd8, 0xa4, 0x18, 0x1a,
				// element: [uint64(15), uint64(30)]
				0x82, 0xd8, 0xa4, 0x0f, 0xd8, 0xa4, 0x18, 0x1e,
				// element: [uint64(17), uint64(34)]
				0x82, 0xd8, 0xa4, 0x11, 0xd8, 0xa4, 0x18, 0x22,
				// element: [uint64(19), uint64(38)]
				0x82, 0xd8, 0xa4, 0x13, 0xd8, 0xa4, 0x18, 0x26,
			},
		}

		stored, err := storage.Encode()
		require.NoError(t, err)

		require.Equal(t, expected[id1], stored[id1])
		require.Equal(t, expected[id2], stored[id2])
		require.Equal(t, expected[id3], stored[id3])

		// Decode data to new storage
		storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		err = storage2.Load(stored)
		require.NoError(t, err)

		// Test new map from storage2
		decodedMap, err := NewMapWithRootID(storage2, id1, digesterBuilder)
		require.NoError(t, err)

		require.Equal(t, uint64(mapSize), decodedMap.Count())
		require.Equal(t, typeInfo, decodedMap.Type())

		for i := uint64(0); i < mapSize; i++ {
			s, err := decodedMap.Get(compare, hashInputProvider, Uint64Value(i))
			require.NoError(t, err)

			v, err := s.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, Uint64Value(i*2), v)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		// Create and populate map in memory
		storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

		digesterBuilder := &mockDigesterBuilder{}

		// Create map
		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		k := Uint64Value(0)
		v := Uint64Value(0)

		digests := []Digest{Digest(0), Digest(1)}
		digesterBuilder.On("Digest", k).Return(mockDigester{d: digests})

		existingStorable, err := m.Set(compare, hashInputProvider, k, v)
		require.NoError(t, err)
		require.Nil(t, existingStorable)

		require.Equal(t, uint64(1), m.Count())

		id1 := StorageID{Address: address, Index: StorageIndex{0, 0, 0, 0, 0, 0, 0, 1}}

		expectedNoPointer := []byte{

			// version
			0x00,
			// flag: root + map data
			0x88,
			// extra data (CBOR encoded array of 3 elements)
			0x83,
			// type info: "map"
			//0x63, 0x6d, 0x61, 0x70,
			0x18, 0x2A,
			// count: 10
			0x01,
			// seed
			0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

			// version
			0x00,
			// flag: root + map data
			0x88,

			// the following encoded data is valid CBOR

			// elements (array of 3 elements)
			0x83,

			// level: 0
			0x00,

			// hkeys (byte string of length 8 * 1)
			0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
			// hkey: 0
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

			// elements (array of 1 elements)
			// each element is encoded as CBOR array of 2 elements (key, value)
			0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
			// element: [uint64(0), uint64(0)]
			0x82, 0xd8, 0xa4, 0x00, 0xd8, 0xa4, 0x00,
		}

		// Verify encoded data
		stored, err := storage.Encode()
		require.NoError(t, err)

		require.Equal(t, expectedNoPointer, stored[id1])

		// Overwrite existing value with long string
		vs := NewStringValue(randStr(512))
		existingStorable, err = m.Set(compare, hashInputProvider, k, vs)
		require.NoError(t, err)

		existingValue, err := existingStorable.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, existingValue)

		expectedHasPointer := []byte{

			// version
			0x00,
			// flag: root + pointer + map data
			0xc8,
			// extra data (CBOR encoded array of 3 elements)
			0x83,
			// type info: "map"
			//0x63, 0x6d, 0x61, 0x70,
			0x18, 0x2A,
			// count: 10
			0x01,
			// seed
			0x1b, 0x52, 0xa8, 0x78, 0x3, 0x85, 0x2c, 0xaa, 0x49,

			// version
			0x00,
			// flag: root + pointer + map data
			0xc8,

			// the following encoded data is valid CBOR

			// elements (array of 3 elements)
			0x83,

			// level: 0
			0x00,

			// hkeys (byte string of length 8 * 1)
			0x5b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08,
			// hkey: 0
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,

			// elements (array of 1 elements)
			// each element is encoded as CBOR array of 2 elements (key, value)
			0x9b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
			// element: [uint64(0), storage id]
			0x82, 0xd8, 0xa4, 0x00,
			// (tag content: storage id)
			0xd8, 0xff, 0x50,
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
		}

		stored, err = storage.Encode()
		require.NoError(t, err)

		require.Equal(t, expectedHasPointer, stored[id1])

	})
}

func TestMapEncodeDecodeRandomData(t *testing.T) {

	const (
		SetAction = iota
		RemoveAction
		MaxAction
	)

	const (
		Uint8Type = iota
		Uint16Type
		Uint32Type
		Uint64Type
		StringType
		MaxType
	)

	SetThreshold(256)
	defer func() {
		SetThreshold(1024)
	}()

	const actionCount = 2 * 1024

	const stringMaxSize = 512

	typeInfo := testTypeInfo{42}

	encMode, err := cbor.EncOptions{}.EncMode()
	require.NoError(t, err)

	decMode, err := cbor.DecOptions{}.DecMode()
	require.NoError(t, err)

	storage := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	digesterBuilder := newBasicDigesterBuilder()

	m, err := NewMap(storage, address, digesterBuilder, typeInfo)
	require.NoError(t, err)

	keyValues := make(map[Value]Value)
	var keys []Value

	for i := uint64(0); i < actionCount; i++ {

		action := SetAction

		if len(keys) > 0 {
			action = rand.Intn(MaxAction)
		}

		switch action {

		case SetAction:

			var k Value

			switch rand.Intn(MaxType) {
			case Uint8Type:
				n := rand.Intn(math.MaxUint8 + 1)
				k = Uint8Value(n)
			case Uint16Type:
				n := rand.Intn(math.MaxUint16 + 1)
				k = Uint16Value(n)
			case Uint32Type:
				k = Uint32Value(rand.Uint32())
			case Uint64Type:
				k = Uint64Value(rand.Uint64())
			case StringType:
				k = NewStringValue(randStr(rand.Intn(stringMaxSize)))
			}

			var v Value

			switch rand.Intn(MaxType) {
			case Uint8Type:
				n := rand.Intn(math.MaxUint8 + 1)
				v = Uint8Value(n)
			case Uint16Type:
				n := rand.Intn(math.MaxUint16 + 1)
				v = Uint16Value(n)
			case Uint32Type:
				v = Uint32Value(rand.Uint32())
			case Uint64Type:
				v = Uint64Value(rand.Uint64())
			case StringType:
				v = NewStringValue(randStr(rand.Intn(stringMaxSize)))
			}

			keyValues[k] = v

			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)

			if existingStorable == nil {
				keys = append(keys, k)
			}

		case RemoveAction:

			ki := rand.Intn(len(keys))
			k := keys[ki]

			removedKeyStorable, removedValueStorable, err := m.Remove(compare, hashInputProvider, k)
			require.NoError(t, err)

			removedKey, err := removedKeyStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, k, removedKey)

			removedValue, err := removedValueStorable.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, keyValues[k], removedValue)

			delete(keyValues, k)
			copy(keys[ki:], keys[ki+1:])
			keys = keys[:len(keys)-1]
		}

		require.Equal(t, m.Count(), uint64(len(keys)))
		require.Equal(t, typeInfo, m.Type())
	}

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	rootID := m.StorageID()

	// Encode slabs with random data of mixed types
	encodedData, err := storage.Encode()
	require.NoError(t, err)

	// Decode data to new storage
	storage2 := NewBasicSlabStorage(encMode, decMode, decodeStorable, decodeTypeInfo)

	err = storage2.Load(encodedData)
	require.NoError(t, err)

	// Create new map from new storage
	m2, err := NewMapWithRootID(storage2, rootID, digesterBuilder)
	require.NoError(t, err)

	require.Equal(t, typeInfo, m2.Type())
	require.Equal(t, uint64(len(keys)), m2.Count())

	// Get and check every element from new map.

	for k, v := range keyValues {
		s, err := m2.Get(compare, hashInputProvider, k)
		require.NoError(t, err)

		e, err := s.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, e)
	}
}

func TestMapStoredValue(t *testing.T) {

	const mapSize = 64 * 1024

	typeInfo := testTypeInfo{42}

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	storage := newTestInMemoryStorage(t)

	uniqueKeys := make(map[string]bool, mapSize)
	uniqueKeyValues := make(map[Value]Value, mapSize)
	for i := uint64(0); i < mapSize; i++ {
		for {
			s := randStr(16)
			if !uniqueKeys[s] {
				uniqueKeys[s] = true

				k := NewStringValue(s)
				uniqueKeyValues[k] = Uint64Value(i)
				break
			}
		}
	}

	m, err := NewMap(storage, address, newBasicDigesterBuilder(), typeInfo)
	require.NoError(t, err)

	for k, v := range uniqueKeyValues {
		existingStorable, err := m.Set(compare, hashInputProvider, k, v)
		require.NoError(t, err)
		require.Nil(t, existingStorable)
	}

	value, err := m.root.StoredValue(storage)
	require.NoError(t, err)

	m2, ok := value.(*OrderedMap)
	require.True(t, ok)

	require.Equal(t, typeInfo, m2.Type())
	require.Equal(t, uint64(len(uniqueKeyValues)), m2.Count())

	for k, v := range uniqueKeyValues {
		strv := k.(StringValue)
		require.NotNil(t, strv)

		s, err := m2.Get(compare, hashInputProvider, NewStringValue(strv.str))
		require.NoError(t, err)

		e, err := s.StoredValue(storage)
		require.NoError(t, err)
		require.Equal(t, v, e)
	}

	firstDataSlab, err := firstMapDataSlab(storage, m.root)
	require.NoError(t, err)

	if firstDataSlab.ID() != m.StorageID() {
		_, err = firstDataSlab.StoredValue(storage)
		require.Error(t, err)
	}
}

func TestMapPopIterate(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		typeInfo := testTypeInfo{42}

		storage := newTestInMemoryStorage(t)

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		digesterBuilder := newBasicDigesterBuilder()

		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)
		require.Equal(t, 1, storage.Count())

		i := uint64(0)
		err = m.PopIterate(func(k Storable, v Storable) {
			i++
		})
		require.NoError(t, err)
		require.Equal(t, uint64(0), i)

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		require.Equal(t, uint64(0), m.Count())
		require.Equal(t, 1, storage.Count())
	})

	t.Run("root-dataslab", func(t *testing.T) {
		SetThreshold(1024)

		const mapSize = 10

		typeInfo := testTypeInfo{42}

		storage := newTestBasicStorage(t)

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		digesterBuilder := newBasicDigesterBuilder()

		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		uniqueKeyValues := make(map[Value]Value, mapSize)

		sortedKeys := make([]Value, mapSize)

		for i := uint64(0); i < mapSize; i++ {
			key, value := Uint64Value(i), Uint64Value(i*10)

			sortedKeys[i] = key

			uniqueKeyValues[key] = value

			_, err := m.Set(compare, hashInputProvider, key, value)
			require.NoError(t, err)
		}

		require.Equal(t, uint64(mapSize), m.Count())
		require.Equal(t, 1, storage.Count())

		sortedKeys = sortSliceByDigest(t, sortedKeys, digesterBuilder, hashInputProvider)

		i := mapSize
		err = m.PopIterate(func(k, v Storable) {
			i--

			kv, err := k.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, sortedKeys[i], kv)

			vv, err := v.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, uniqueKeyValues[sortedKeys[i]], vv)
		})

		require.NoError(t, err)
		require.Equal(t, 0, i)

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		require.Equal(t, uint64(0), m.Count())
		require.Equal(t, typeInfo, m.Type())
		require.Equal(t, 1, storage.Count())
	})

	t.Run("root-metaslab", func(t *testing.T) {
		const mapSize = 64 * 1024

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		storage := newTestInMemoryStorage(t)

		digesterBuilder := newBasicDigesterBuilder()

		uniqueKeyValues := make(map[Value]Value, mapSize)

		sortedKeys := make([]Value, mapSize)

		for i := uint64(0); i < mapSize; i++ {
			for {
				k := NewStringValue(randStr(16))
				if _, exist := uniqueKeyValues[k]; !exist {
					sortedKeys[i] = k
					uniqueKeyValues[k] = NewStringValue(randStr(16))
					break
				}
			}
		}

		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		for k, v := range uniqueKeyValues {
			existingStorable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		sortedKeys = sortSliceByDigest(t, sortedKeys, digesterBuilder, hashInputProvider)

		// Iterate key value pairs
		i := len(uniqueKeyValues)
		err = m.PopIterate(func(k Storable, v Storable) {
			i--

			kv, err := k.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, sortedKeys[i], kv)

			vv, err := v.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, uniqueKeyValues[sortedKeys[i]], vv)
		})

		require.NoError(t, err)
		require.Equal(t, 0, i)

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		require.Equal(t, uint64(0), m.Count())
		require.Equal(t, typeInfo, m.Type())
		require.Equal(t, 1, storage.Count())
	})

	t.Run("collision", func(t *testing.T) {
		//MetaDataSlabCount:1 DataSlabCount:13 CollisionDataSlabCount:100

		const mapSize = 1024

		SetThreshold(512)
		defer SetThreshold(1024)

		typeInfo := testTypeInfo{42}

		address := Address{1, 2, 3, 4, 5, 6, 7, 8}

		digesterBuilder := &mockDigesterBuilder{}

		storage := newTestInMemoryStorage(t)

		uniqueKeyValues := make(map[Value]Value, mapSize)

		sortedKeys := make([]Value, mapSize)

		// keys is needed to insert elements with collision in deterministic order.
		keys := make([]Value, mapSize)

		for i := uint64(0); i < mapSize; i++ {
			for {
				k := NewStringValue(randStr(16))

				if _, ok := uniqueKeyValues[k]; !ok {

					sortedKeys[i] = k
					keys[i] = k
					uniqueKeyValues[k] = NewStringValue(randStr(16))

					digests := []Digest{
						Digest(i % 100),
						Digest(i % 5),
					}

					digesterBuilder.On("Digest", k).Return(mockDigester{digests})
					break
				}
			}
		}

		sortedKeys = sortSliceByDigest(t, sortedKeys, digesterBuilder, hashInputProvider)

		m, err := NewMap(storage, address, digesterBuilder, typeInfo)
		require.NoError(t, err)

		// Populate map
		for _, k := range keys {
			existingStorable, err := m.Set(compare, hashInputProvider, k, uniqueKeyValues[k])
			require.NoError(t, err)
			require.Nil(t, existingStorable)
		}

		// Iterate key value pairs
		i := mapSize
		err = m.PopIterate(func(k Storable, v Storable) {
			i--

			kv, err := k.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, sortedKeys[i], kv)

			vv, err := v.StoredValue(storage)
			require.NoError(t, err)
			require.Equal(t, uniqueKeyValues[sortedKeys[i]], vv)
		})

		require.NoError(t, err)
		require.Equal(t, 0, i)

		err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
		if err != nil {
			PrintMap(m)
		}
		require.NoError(t, err)

		require.Equal(t, uint64(0), m.Count())
		require.Equal(t, typeInfo, m.Type())
		require.Equal(t, 1, storage.Count())
	})
}

func sortSliceByDigest(t *testing.T, x []Value, digesterBuilder DigesterBuilder, hip HashInputProvider) []Value {

	sort.SliceStable(x, func(i, j int) bool {
		d1, err := digesterBuilder.Digest(hip, x[i])
		require.NoError(t, err)

		digest1, err := d1.DigestPrefix(d1.Levels())
		require.NoError(t, err)

		d2, err := digesterBuilder.Digest(hip, x[j])
		require.NoError(t, err)

		digest2, err := d2.DigestPrefix(d2.Levels())
		require.NoError(t, err)

		for z := 0; z < len(digest1); z++ {
			if digest1[z] != digest2[z] {
				return digest1[z] < digest2[z] // sort by hkey
			}
		}
		return i < j // sort by insertion order with hash collision
	})

	return x
}

func TestEmptyMap(t *testing.T) {

	t.Parallel()

	typeInfo := testTypeInfo{42}

	storage := newTestPersistentStorage(t)

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	m, err := NewMap(storage, address, NewDefaultDigesterBuilder(), typeInfo)
	require.NoError(t, err)

	rootID := m.root.Header().id

	err = storage.Commit()
	require.NoError(t, err)

	storage.DropCache()
	storage.DropDeltas()

	t.Run("get", func(t *testing.T) {
		s, err := m.Get(compare, hashInputProvider, Uint64Value(0))
		require.Error(t, err, KeyNotFoundError{})
		require.Nil(t, s)
	})

	t.Run("remove", func(t *testing.T) {
		existingKey, existingValue, err := m.Remove(compare, hashInputProvider, Uint64Value(0))
		require.Error(t, err, KeyNotFoundError{})
		require.Nil(t, existingKey)
		require.Nil(t, existingValue)
	})

	t.Run("iterate", func(t *testing.T) {
		i := uint64(0)
		err := m.Iterate(func(k Value, v Value) (bool, error) {
			i++
			return true, nil
		})
		require.NoError(t, err)
		require.Equal(t, uint64(0), i)
	})

	t.Run("count", func(t *testing.T) {
		count := m.Count()
		require.Equal(t, uint64(0), count)
	})

	t.Run("type", func(t *testing.T) {
		require.Equal(t, typeInfo, m.Type())
	})

	t.Run("decode", func(t *testing.T) {
		m2, err := NewMapWithRootID(storage, rootID, newBasicDigesterBuilder())
		require.NoError(t, err)
		require.True(t, m2.root.IsData())
		require.Equal(t, uint64(0), m2.Count())
		require.Equal(t, typeInfo, m2.Type())
		require.Equal(t, uint32(mapRootDataSlabPrefixSize+hkeyElementsPrefixSize), m2.root.Header().size)
	})
}

func TestMapBatchSet(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		typeInfo := testTypeInfo{42}

		m, err := NewMap(
			newTestBasicStorage(t),
			Address{1, 2, 3, 4, 5, 6, 7, 8},
			NewDefaultDigesterBuilder(),
			typeInfo,
		)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.Count())
		require.Equal(t, typeInfo, m.Type())

		iter, err := m.Iterator()
		require.NoError(t, err)

		storage1 := newTestBasicStorage(t)

		// Create a new map with new storage, new address, and original map's elements.
		copied, err := NewMapFromBatchData(
			storage1,
			Address{2, 3, 4, 5, 6, 7, 8, 9},
			NewDefaultDigesterBuilder(),
			m.Type(),
			compare,
			hashInputProvider,
			m.Seed(),
			func() (Value, Value, error) {
				return iter.Next()
			})
		require.NoError(t, err)
		require.Equal(t, uint64(0), copied.Count())
		require.Equal(t, m.Type(), copied.Type())
		require.NotEqual(t, copied.StorageID(), m.StorageID())

		// Encode copied map
		encoded, err := storage1.Encode()
		require.NoError(t, err)

		// Load encoded slabs into a new storage
		storage2 := newTestBasicStorage(t)
		err = storage2.Load(encoded)
		require.NoError(t, err)

		testPopulatedMapFromStorage(t, storage2, copied.StorageID(), typeInfo, NewDefaultDigesterBuilder(), compare, hashInputProvider, nil, nil)
	})

	t.Run("root-dataslab", func(t *testing.T) {
		SetThreshold(1024)

		const mapSize = 10

		typeInfo := testTypeInfo{42}

		m, err := NewMap(
			newTestBasicStorage(t),
			Address{1, 2, 3, 4, 5, 6, 7, 8},
			NewDefaultDigesterBuilder(),
			typeInfo,
		)
		require.NoError(t, err)

		for i := uint64(0); i < mapSize; i++ {
			storable, err := m.Set(compare, hashInputProvider, Uint64Value(i), Uint64Value(i*10))
			require.NoError(t, err)
			require.Nil(t, storable)
		}

		require.Equal(t, uint64(mapSize), m.Count())
		require.Equal(t, typeInfo, m.Type())

		iter, err := m.Iterator()
		require.NoError(t, err)

		var sortedKeys []Value
		keyValues := make(map[Value]Value)

		storage1 := newTestBasicStorage(t)

		digesterBuilder := NewDefaultDigesterBuilder()

		// Create a new map with new storage, new address, and original map's elements.
		copied, err := NewMapFromBatchData(
			storage1,
			Address{2, 3, 4, 5, 6, 7, 8, 9},
			digesterBuilder,
			m.Type(),
			compare,
			hashInputProvider,
			m.Seed(),
			func() (Value, Value, error) {

				k, v, err := iter.Next()

				// Save key value pair
				if k != nil {
					sortedKeys = append(sortedKeys, k)
					keyValues[k] = v
				}

				return k, v, err
			})

		require.NoError(t, err)
		require.Equal(t, uint64(mapSize), copied.Count())
		require.Equal(t, typeInfo, copied.Type())
		require.NotEqual(t, copied.StorageID(), m.StorageID())

		// Encode copied map
		encoded, err := storage1.Encode()
		require.NoError(t, err)

		// Load encoded slabs into a new storage
		storage2 := newTestBasicStorage(t)
		err = storage2.Load(encoded)
		require.NoError(t, err)

		testPopulatedMapFromStorage(t, storage2, copied.StorageID(), typeInfo, NewDefaultDigesterBuilder(), compare, hashInputProvider, sortedKeys, keyValues)
	})

	t.Run("root-metaslab", func(t *testing.T) {
		SetThreshold(100)
		defer func() {
			SetThreshold(1024)
		}()

		const mapSize = 1024 * 64

		typeInfo := testTypeInfo{42}

		m, err := NewMap(
			newTestBasicStorage(t),
			Address{1, 2, 3, 4, 5, 6, 7, 8},
			NewDefaultDigesterBuilder(),
			typeInfo,
		)
		require.NoError(t, err)

		for i := uint64(0); i < mapSize; i++ {
			storable, err := m.Set(compare, hashInputProvider, Uint64Value(i), Uint64Value(i*10))
			require.NoError(t, err)
			require.Nil(t, storable)
		}

		require.Equal(t, uint64(mapSize), m.Count())
		require.Equal(t, typeInfo, m.Type())

		iter, err := m.Iterator()
		require.NoError(t, err)

		var sortedKeys []Value
		keyValues := make(map[Value]Value)

		storage1 := newTestBasicStorage(t)

		digesterBuilder := NewDefaultDigesterBuilder()

		copied, err := NewMapFromBatchData(
			storage1,
			Address{2, 3, 4, 5, 6, 7, 8, 9},
			digesterBuilder,
			m.Type(),
			compare,
			hashInputProvider,
			m.Seed(),
			func() (Value, Value, error) {
				k, v, err := iter.Next()

				if k != nil {
					sortedKeys = append(sortedKeys, k)
					keyValues[k] = v
				}

				return k, v, err
			})

		require.NoError(t, err)
		require.Equal(t, uint64(mapSize), copied.Count())
		require.Equal(t, typeInfo, copied.Type())
		require.NotEqual(t, m.StorageID(), copied.StorageID())

		// Encode copied map
		encoded, err := storage1.Encode()
		require.NoError(t, err)

		// Load encoded slabs into a new storage
		storage2 := newTestBasicStorage(t)
		err = storage2.Load(encoded)
		require.NoError(t, err)

		testPopulatedMapFromStorage(t, storage2, copied.StorageID(), typeInfo, NewDefaultDigesterBuilder(), compare, hashInputProvider, sortedKeys, keyValues)
	})

	t.Run("random", func(t *testing.T) {
		SetThreshold(100)
		defer func() {
			SetThreshold(1024)
		}()

		const mapSize = 1024 * 64

		typeInfo := testTypeInfo{42}

		m, err := NewMap(
			newTestBasicStorage(t),
			Address{1, 2, 3, 4, 5, 6, 7, 8},
			NewDefaultDigesterBuilder(),
			typeInfo,
		)
		require.NoError(t, err)

		for m.Count() < mapSize {
			k := RandomValue()
			v := RandomValue()

			_, err = m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
		}

		require.Equal(t, uint64(mapSize), m.Count())
		require.Equal(t, typeInfo, m.Type())

		iter, err := m.Iterator()
		require.NoError(t, err)

		storage1 := newTestBasicStorage(t)

		digesterBuilder := NewDefaultDigesterBuilder()

		var sortedKeys []Value
		keyValues := make(map[Value]Value, mapSize)

		copied, err := NewMapFromBatchData(
			storage1,
			Address{2, 3, 4, 5, 6, 7, 8, 9},
			digesterBuilder,
			m.Type(),
			compare,
			hashInputProvider,
			m.Seed(),
			func() (Value, Value, error) {
				k, v, err := iter.Next()

				if k != nil {
					sortedKeys = append(sortedKeys, k)
					keyValues[k] = v
				}

				return k, v, err
			})

		require.NoError(t, err)
		require.Equal(t, uint64(mapSize), copied.Count())
		require.Equal(t, typeInfo, copied.Type())
		require.NotEqual(t, m.StorageID(), copied.StorageID())

		// Encode copied map
		encoded, err := storage1.Encode()
		require.NoError(t, err)

		// Load encoded slabs into a new storage
		storage2 := newTestBasicStorage(t)
		err = storage2.Load(encoded)
		require.NoError(t, err)

		testPopulatedMapFromStorage(t, storage2, copied.StorageID(), typeInfo, NewDefaultDigesterBuilder(), compare, hashInputProvider, sortedKeys, keyValues)
	})

	t.Run("collision", func(t *testing.T) {

		SetThreshold(512)
		defer func() {
			SetThreshold(1024)
		}()

		const mapSize = 1024

		typeInfo := testTypeInfo{42}

		digesterBuilder := &mockDigesterBuilder{}

		m, err := NewMap(
			newTestBasicStorage(t),
			Address{1, 2, 3, 4, 5, 6, 7, 8},
			digesterBuilder,
			typeInfo,
		)
		require.NoError(t, err)

		for i := uint64(0); i < mapSize; i++ {

			k, v := Uint64Value(i), Uint64Value(i*10)

			digests := make([]Digest, 2)
			if i%2 == 0 {
				digests[0] = 0
			} else {
				digests[0] = Digest(i % (mapSize / 2))
			}
			digests[1] = Digest(i)

			digesterBuilder.On("Digest", k).Return(mockDigester{digests})

			storable, err := m.Set(compare, hashInputProvider, k, v)
			require.NoError(t, err)
			require.Nil(t, storable)
		}

		require.Equal(t, uint64(mapSize), m.Count())
		require.Equal(t, typeInfo, m.Type())

		iter, err := m.Iterator()
		require.NoError(t, err)

		var sortedKeys []Value
		keyValues := make(map[Value]Value)

		storage1 := newTestBasicStorage(t)

		i := 0
		copied, err := NewMapFromBatchData(
			storage1,
			Address{2, 3, 4, 5, 6, 7, 8, 9},
			digesterBuilder,
			m.Type(),
			compare,
			hashInputProvider,
			m.Seed(),
			func() (Value, Value, error) {
				k, v, err := iter.Next()

				if k != nil {
					sortedKeys = append(sortedKeys, k)
					keyValues[k] = v
				}

				i++
				return k, v, err
			})

		require.NoError(t, err)
		require.Equal(t, uint64(mapSize), copied.Count())
		require.Equal(t, typeInfo, copied.Type())
		require.NotEqual(t, m.StorageID(), copied.StorageID())

		// Encode copied map
		encoded, err := storage1.Encode()
		require.NoError(t, err)

		// Load encoded slabs into a new storage
		storage2 := newTestBasicStorage(t)
		err = storage2.Load(encoded)
		require.NoError(t, err)

		testPopulatedMapFromStorage(
			t,
			storage2,
			copied.StorageID(),
			typeInfo,
			digesterBuilder,
			compare,
			hashInputProvider,
			sortedKeys,
			keyValues,
		)
	})

}

func testPopulatedMapFromStorage(
	t *testing.T,
	storage *BasicSlabStorage,
	rootID StorageID,
	typeInfo TypeInfo,
	digesterBuilder DigesterBuilder,
	comparator Comparator,
	hip HashInputProvider,
	sortedKeys []Value,
	keyValues map[Value]Value,
) {

	m, err := NewMapWithRootID(storage, rootID, digesterBuilder)
	require.NoError(t, err)

	// Get map's elements to test tree traversal.
	for k, v := range keyValues {
		storable, err := m.Get(comparator, hip, k)
		require.NoError(t, err)
		require.Equal(t, v, storable)
	}

	// Iterate through map to test data slab's next
	i := 0
	err = m.Iterate(func(k, v Value) (bool, error) {
		expectedKey := sortedKeys[i]
		expectedValue := keyValues[expectedKey]

		require.Equal(t, expectedKey, k)
		require.Equal(t, expectedValue, v)

		i++
		return true, nil
	})
	require.NoError(t, err)
	require.Equal(t, len(keyValues), i)

	err = ValidMap(m, typeInfo, typeInfoComparator, hashInputProvider)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)

	err = ValidMapSerialization(m, storage.cborDecMode, storage.cborEncMode, storage.DecodeStorable, storage.DecodeTypeInfo)
	if err != nil {
		PrintMap(m)
	}
	require.NoError(t, err)
}
