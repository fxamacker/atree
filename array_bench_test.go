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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// var slabTargetSize = flag.Int("slabsize", 1024, "target slab size")

var noop Storable

func BenchmarkGetXSArray(b *testing.B) { benchmarkGet(b, 10, 100) }

func BenchmarkGetSArray(b *testing.B) { benchmarkGet(b, 1000, 100) }

func BenchmarkGetMArray(b *testing.B) { benchmarkGet(b, 10_000, 100) }

func BenchmarkGetLArray(b *testing.B) { benchmarkGet(b, 100_000, 100) }

func BenchmarkGetXLArray(b *testing.B) { benchmarkGet(b, 1_000_000, 100) }

func BenchmarkGetXXLArray(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping BenchmarkGetXXLArray in short mode")
	}
	benchmarkGet(b, 10_000_000, 100)
}

func BenchmarkInsertXSArray(b *testing.B) { benchmarkInsert(b, 10, 100) }

func BenchmarkInsertSArray(b *testing.B) { benchmarkInsert(b, 1000, 100) }

func BenchmarkInsertMArray(b *testing.B) { benchmarkInsert(b, 10_000, 100) }

func BenchmarkInsertLArray(b *testing.B) { benchmarkInsert(b, 100_000, 100) }

func BenchmarkInsertXLArray(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping BenchmarkInsertXLArray in short mode")
	}
	benchmarkInsert(b, 1_000_000, 100)
}

func BenchmarkInsertXXLArray(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping BenchmarkInsertXXLArray in short mode")
	}
	benchmarkInsert(b, 10_000_000, 100)
}

func BenchmarkRemoveXSArray(b *testing.B) { benchmarkRemove(b, 10, 10) }

func BenchmarkRemoveSArray(b *testing.B) { benchmarkRemove(b, 1000, 100) }

func BenchmarkRemoveMArray(b *testing.B) { benchmarkRemove(b, 10_000, 100) }

func BenchmarkRemoveLArray(b *testing.B) { benchmarkRemove(b, 100_000, 100) }

func BenchmarkRemoveXLArray(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping BenchmarkRemoveXLArray in short mode")
	}
	benchmarkRemove(b, 1_000_000, 100)
}

func BenchmarkRemoveXXLArray(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping BenchmarkRemoveXXLArray in short mode")
	}
	benchmarkRemove(b, 10_000_000, 100)
}

func BenchmarkBatchRemoveArray100Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopRemove(b, 100)
	})
	b.Run("pop", func(b *testing.B) {
		benchmarkPopRemove(b, 100)
	})
}

func BenchmarkBatchRemoveArray1000Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopRemove(b, 1000)
	})
	b.Run("pop", func(b *testing.B) {
		benchmarkPopRemove(b, 1000)
	})
}

func BenchmarkBatchRemoveArray10000Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopRemove(b, 10_000)
	})
	b.Run("pop", func(b *testing.B) {
		benchmarkPopRemove(b, 10_000)
	})
}

func BenchmarkBatchRemoveArray100000Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopRemove(b, 100_000)
	})
	b.Run("pop", func(b *testing.B) {
		benchmarkPopRemove(b, 100_000)
	})
}

func BenchmarkBatchAppendArray100Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopAppend(b, 100)
	})
	b.Run("batch", func(b *testing.B) {
		benchmarkBatchAppend(b, 100)
	})
}

func BenchmarkBatchAppendArray1000Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopAppend(b, 1000)
	})
	b.Run("batch", func(b *testing.B) {
		benchmarkBatchAppend(b, 1000)
	})
}

func BenchmarkBatchAppendArray10000Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopAppend(b, 10_000)
	})
	b.Run("batch", func(b *testing.B) {
		benchmarkBatchAppend(b, 10_000)
	})
}

func BenchmarkBatchAppendArray100000Elems(b *testing.B) {
	b.Run("loop", func(b *testing.B) {
		benchmarkLoopAppend(b, 100_000)
	})
	b.Run("batch", func(b *testing.B) {
		benchmarkBatchAppend(b, 100_000)
	})
}

// XXXLArray takes too long to run.
// func BenchmarkLookupXXXLArray(b *testing.B) { benchmarkLookup(b, 100_000_000, 100) }

func setupArray(storage *PersistentSlabStorage, initialArraySize int) (*Array, error) {

	address := Address{1, 2, 3, 4, 5, 6, 7, 8}

	typeInfo := testTypeInfo{42}

	array, err := NewArray(storage, address, typeInfo)
	if err != nil {
		return nil, err
	}

	for i := 0; i < initialArraySize; i++ {
		v := RandomValue()
		err := array.Append(v)
		if err != nil {
			return nil, err
		}
	}

	err = storage.Commit()
	if err != nil {
		return nil, err
	}

	arrayID := array.StorageID()

	storage.DropCache()

	newArray, err := NewArrayWithRootID(storage, arrayID)
	if err != nil {
		return nil, err
	}

	return newArray, nil
}

func benchmarkGet(b *testing.B, initialArraySize, numberOfElements int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	array, err := setupArray(storage, initialArraySize)
	require.NoError(b, err)

	var storable Storable

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for i := 0; i < numberOfElements; i++ {
			index := rand.Intn(int(array.Count()))
			storable, _ = array.Get(uint64(index))
		}
	}

	noop = storable
}

func benchmarkInsert(b *testing.B, initialArraySize, numberOfElements int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	for i := 0; i < b.N; i++ {

		b.StopTimer()

		array, err := setupArray(storage, initialArraySize)
		require.NoError(b, err)

		b.StartTimer()

		for i := 0; i < numberOfElements; i++ {
			index := rand.Intn(int(array.Count()))
			v := RandomValue()
			_ = array.Insert(uint64(index), v)
		}
	}
}

func benchmarkRemove(b *testing.B, initialArraySize, numberOfElements int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	for i := 0; i < b.N; i++ {

		b.StopTimer()

		array, err := setupArray(storage, initialArraySize)
		require.NoError(b, err)

		b.StartTimer()

		for i := 0; i < numberOfElements; i++ {
			index := rand.Intn(int(array.Count()))
			_, _ = array.Remove(uint64(index))
		}
	}
}

func benchmarkLoopRemove(b *testing.B, initialArraySize int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	array, err := setupArray(storage, initialArraySize)
	require.NoError(b, err)

	var storable Storable

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for i := initialArraySize - 1; i >= 0; i-- {
			storable, _ = array.Remove(uint64(i))
		}
	}

	noop = storable
}

func benchmarkPopRemove(b *testing.B, initialArraySize int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	array, err := setupArray(storage, initialArraySize)
	require.NoError(b, err)

	var storable Storable

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		err := array.PopIterate(func(s Storable) {
			storable = s
		})
		if err != nil {
			b.Errorf(err.Error())
		}
	}

	noop = storable
}

func benchmarkLoopAppend(b *testing.B, initialArraySize int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	array, err := setupArray(storage, initialArraySize)
	require.NoError(b, err)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		copied, _ := NewArray(storage, array.Address(), array.Type())

		_ = array.Iterate(func(value Value) (bool, error) {
			err = copied.Append(value)
			return true, nil
		})

		if copied.Count() != array.Count() {
			b.Errorf("Copied array has %d elements, want %d", copied.Count(), array.Count())
		}
	}
}

func benchmarkBatchAppend(b *testing.B, initialArraySize int) {

	b.StopTimer()

	storage := newTestPersistentStorage(b)

	array, err := setupArray(storage, initialArraySize)
	require.NoError(b, err)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		iter, err := array.Iterator()
		require.NoError(b, err)

		copied, _ := NewArrayFromBatchData(storage, array.Address(), array.Type(), func() (Value, error) {
			return iter.Next()
		})

		if copied.Count() != array.Count() {
			b.Errorf("Copied array has %d elements, want %d", copied.Count(), array.Count())
		}
	}
}
