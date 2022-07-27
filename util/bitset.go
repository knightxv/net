package util

const (
	shift = 6    // 2 ^ 6 = 64
	mask  = 0x3f // 63
)

type BitSet struct {
	Data []uint64
	Size int
}

func NewBitSet(size int) *BitSet {
	return &BitSet{
		Data: make([]uint64, (size>>shift)+1),
		Size: size,
	}
}

func index(i int) int {
	return i >> shift
}

func offset(i int) int {
	return i & mask
}

func posVal(i int) uint64 {
	return 1 << uint(offset(i))
}

func (b *BitSet) checkValid(n int) {
	if n < 0 || n >= b.Size {
		panic("index out of range")
	}
}

func (b *BitSet) Set(n int) *BitSet {
	b.checkValid(n)
	b.Data[index(n)] |= posVal(n)
	return b
}

func (b *BitSet) Clear(n int) *BitSet {
	b.checkValid(n)
	b.Data[index(n)] &= ^posVal(n)
	return b
}

// func (b *BitSet) Value() []uint64 {
// 	return b.Data
// }

func (b *BitSet) IsSet(n int) bool {
	b.checkValid(n)
	return b.Data[index(n)]&(posVal(n)) != 0
}

func (b *BitSet) IsEmpty() bool {
	return b.LowestSetBit() == -1
}

func (b *BitSet) LowestSetBit() int {
	for i := 0; i < b.Size; i++ {
		if b.IsSet(i) {
			return i
		}
	}

	return -1
}

func (b *BitSet) HighestSetBit() int {
	for i := b.Size - 1; i >= 0; i-- {
		if b.IsSet(i) {
			return i
		}
	}

	return -1
}
