package telnet

import (
	"testing"

	"github.com/yuin/gopher-lua"
)

// What have I learned? That there's practically 0 overhead to: byte(tnSeq)
// BenchmarkTypeConversion 2000000000               0.31 ns/op            0 B/op          0 allocs/op
// BenchmarkNoConversion   2000000000               0.40 ns/op            0 B/op          0 allocs/op
//
// func BenchmarkTypeConversion(t *testing.B) {
// 	var a byte = 0xFF
// 	var b int = 0xFF
// 	for i := 0; i < t.N; i++ {
// 		if a == byte(b) {
// 		}
// 	}
// }
//
// func BenchmarkNoConversion(t *testing.B) {
// 	var a byte = 0xFF
// 	var b byte = 0xFF
// 	for i := 0; i < t.N; i++ {
// 		if a == b {
// 		}
// 	}
// }

func TestBToSeq(t *testing.T) {
	tests := []struct {
		b byte
		s tnSeq
	}{
		{b: 0xFF, s: IAC},
		{b: 0xC8, s: ATCP},
		{b: 0xC9, s: GMCP},
	}

	for _, c := range tests {
		res := bToSeq(c.b)
		if res != c.s {
			t.Errorf("bToSeq(0x%X) == %s, wanted %s", c.b, res, c.s)
		}
	}
}

func BenchmarkLuaFib10(b *testing.B) {
	L := lua.NewState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := L.DoString(`local function fib(n)
    if n < 2 then return n end
    return fib(n - 2) + fib(n - 1)
end
fib(10)`); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkLuaFib30(b *testing.B) {
	L := lua.NewState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := L.DoString(`local function fib(n)
    if n < 2 then return n end
    return fib(n - 2) + fib(n - 1)
end
fib(30)`); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkLuaSimple(b *testing.B) {
	L := lua.NewState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := L.DoString(`local i = 1 + 1`); err != nil {
			b.Error(err)
		}
	}
}
