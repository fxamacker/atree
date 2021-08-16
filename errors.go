package atree

import (
	"fmt"
)

type FatalError struct {
	err error
}

func NewFatalError(err error) error {
	return &FatalError{err: err}
}

func (e *FatalError) Error() string {
	return fmt.Sprintf("fatal error: %s", e.err.Error())
}

func (e *FatalError) Unwrap() error { return e.err }

// IndexOutOfBoundsError is returned when an insert or delete operation is attempted on an array index which is out of bounds
type IndexOutOfBoundsError struct {
	index uint64
	min   uint64
	max   uint64
}

// NewIndexOutOfBoundsError constructs a IndexOutOfBoundsError
func NewIndexOutOfBoundsError(index, min, max uint64) *IndexOutOfBoundsError {
	return &IndexOutOfBoundsError{index: index, min: min, max: max}
}

func (e *IndexOutOfBoundsError) Error() string {
	return fmt.Sprintf("the given index %d is not in the acceptable range (%d-%d)", e.index, e.min, e.max)
}

// MaxArraySizeError is returned when an insert or delete operation is attempted on an array which has reached maximum size
type MaxArraySizeError struct {
	maxLen uint64
}

// NewMaxArraySizeError constructs a MaxArraySizeError
func NewMaxArraySizeError(maxLen uint64) *MaxArraySizeError {
	return &MaxArraySizeError{maxLen: maxLen}
}

func (e *MaxArraySizeError) Error() string {
	return fmt.Sprintf("array has reach its maximum number of elements %d", e.maxLen)
}

func (e *MaxArraySizeError) Fatal() error {
	return NewFatalError(e)
}

// NonStorableElementError is returned when we try to store a non-storable element.
type NonStorableElementError struct {
	element interface{}
}

// NonStorableElementError constructs a NonStorableElementError
func NewNonStorableElementError(element interface{}) *NonStorableElementError {
	return &NonStorableElementError{element: element}
}

func (e *NonStorableElementError) Error() string {
	return fmt.Sprintf("a non-storable element of type %T found when storing object", e.element)
}

// MaxKeySizeError is returned when a dictionary key is too large
type MaxKeySizeError struct {
	keyStr     string
	maxKeySize uint64
}

// NewMaxKeySizeError constructs a MaxKeySizeError
func NewMaxKeySizeError(keyStr string, maxKeySize uint64) *MaxKeySizeError {
	return &MaxKeySizeError{keyStr: keyStr, maxKeySize: maxKeySize}
}

func (e *MaxKeySizeError) Error() string {
	return fmt.Sprintf("the given key (%s) is larger than the maximum limit (%d)", e.keyStr, e.maxKeySize)
}

// HashError is a fatal error returned when hash calculation fails
type HashError struct {
	err error
}

// NewHashError constructs a HashError
func NewHashError(err error) *HashError {
	return &HashError{err: err}
}

func (e *HashError) Error() string {
	return fmt.Sprintf("atree hasher failed: %s", e.err.Error())
}

// Unwrap returns the wrapped err
func (e HashError) Unwrap() error {
	return e.err
}

func (e *HashError) Fatal() error {
	return NewFatalError(e)
}

// StorageError is a usually fatal error returned when storage fails
type StorageError struct {
	err error
}

// NewStorageError constructs a StorageError
func NewStorageError(err error) *StorageError {
	return &StorageError{err: err}
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("storage failed: %s", e.err.Error())
}

// Unwrap returns the wrapped err
func (e StorageError) Unwrap() error {
	return e.err
}

func (e *StorageError) Fatal() error {
	return NewFatalError(e)
}

// SlabNotFoundError is a usually fatal error returned when an slab is not found
type SlabNotFoundError struct {
	storageID StorageID
	err       error
}

// NewSlabNotFoundError constructs a SlabNotFoundError
func NewSlabNotFoundError(storageID StorageID, err error) *SlabNotFoundError {
	return &SlabNotFoundError{storageID: storageID, err: err}
}

// NewSlabNotFoundErrorf constructs a new SlabNotFoundError with error formating
func NewSlabNotFoundErrorf(storageID StorageID, msg string, args ...interface{}) *SlabNotFoundError {
	return &SlabNotFoundError{storageID: storageID, err: fmt.Errorf(msg, args...)}
}

func (e *SlabNotFoundError) Error() string {
	return fmt.Sprintf("slab with the given storageID (%s) not found. %s", e.storageID.String(), e.err.Error())
}

func (e *SlabNotFoundError) Fatal() error {
	return NewFatalError(e)
}

// SlabSplitError is a usually fatal error returned when splitting an slab has failed
type SlabSplitError struct {
	err error
}

// NewSlabSplitError constructs a SlabSplitError
func NewSlabSplitError(err error) *SlabSplitError {
	return &SlabSplitError{err: err}
}

// NewSlabSplitErrorf constructs a new SlabSplitError with error formating
func NewSlabSplitErrorf(msg string, args ...interface{}) *SlabSplitError {
	return &SlabSplitError{err: fmt.Errorf(msg, args...)}
}

func (e *SlabSplitError) Error() string {
	return fmt.Sprintf("slab can not split. %s", e.err.Error())
}

func (e *SlabSplitError) Fatal() error {
	return NewFatalError(e)
}

// SlabError is a usually fatal error returned when something is wrong with the content of the slab
type SlabError struct {
	err error
}

// NewSlabError constructs a SlabError
func NewSlabError(err error) *SlabError {
	return &SlabError{err: err}
}

// NewSlabErrorf constructs a new DataSlabError with error formating
func NewSlabErrorf(msg string, args ...interface{}) *SlabError {
	return &SlabError{err: fmt.Errorf(msg, args...)}
}

func (e *SlabError) Error() string {
	return fmt.Sprintf("slab error: %s", e.err.Error())
}

func (e *SlabError) Fatal() error {
	return NewFatalError(e)
}

// WrongSlabTypeFoundError is a usually fatal error returned when an slab is loaded but has an unexpected type
type WrongSlabTypeFoundError struct {
	storageID StorageID
}

// NewWrongSlabTypeFoundError constructs a WrongSlabTypeFoundError
func NewWrongSlabTypeFoundError(storageID StorageID) *WrongSlabTypeFoundError {
	return &WrongSlabTypeFoundError{storageID: storageID}
}

func (e *WrongSlabTypeFoundError) Error() string {
	return fmt.Sprintf("slab with the given storageID (%s) has a wrong type", e.storageID.String())
}

func (e *WrongSlabTypeFoundError) Fatal() error {
	return NewFatalError(e)
}

// DigestLevelNotMatchError is a usually fatal error returned when a digest level in the dictionary is not matched
type DigestLevelNotMatchError struct {
	got      uint8
	expected uint8
}

// NewDigestLevelNotMatchError constructs a DigestLevelNotMatchError
func NewDigestLevelNotMatchError(got, expected uint8) *DigestLevelNotMatchError {
	return &DigestLevelNotMatchError{got: got, expected: expected}
}

func (e *DigestLevelNotMatchError) Error() string {
	return fmt.Sprintf("got digest level of %d but was expecting %d", e.got, e.expected)
}

func (e *DigestLevelNotMatchError) Fatal() error {
	return NewFatalError(e)
}

// EncodingError is a usually fatal error returned when a encoding operation fails
type EncodingError struct {
	err error
}

// NewEncodingError constructs a EncodingError
func NewEncodingError(err error) *EncodingError {
	return &EncodingError{err: err}
}

// NewEncodingErrorf constructs a new EncodingError with error formating
func NewEncodingErrorf(msg string, args ...interface{}) *EncodingError {
	return &EncodingError{err: fmt.Errorf(msg, args...)}
}

func (e *EncodingError) Error() string {
	return fmt.Sprintf("Encoding has failed %s", e.err.Error())
}

func (e *EncodingError) Fatal() error {
	return NewFatalError(e)
}

// DecodingError is a usually fatal error returned when a decoding operation fails
type DecodingError struct {
	err error
}

// NewDecodingError constructs a DecodingError
func NewDecodingError(err error) *DecodingError {
	return &DecodingError{err: err}
}

// NewDecodingErrorf constructs a new DecodingError with error formating
func NewDecodingErrorf(msg string, args ...interface{}) *DecodingError {
	return &DecodingError{err: fmt.Errorf(msg, args...)}
}

func (e *DecodingError) Error() string {
	return fmt.Sprintf("Encoding has failed %s", e.err.Error())
}

func (e *DecodingError) Fatal() error {
	return NewFatalError(e)
}
