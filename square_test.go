package my_chess_game

import (
	"testing"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

// As duas convenções de coordenada são invertidas, então este teste vem antes
// de qualquer outra coisa do motor.
func TestSquareIndex(t *testing.T) {
	tests := []struct {
		name string
		sq   parser.Square
		want int
	}{
		{"a8 é o índice 0", parser.Square{File: 0, Rank: 7}, 0},
		{"h8", parser.Square{File: 7, Rank: 7}, 7},
		{"a1", parser.Square{File: 0, Rank: 0}, 56},
		{"h1 é o índice 63", parser.Square{File: 7, Rank: 0}, 63},
		{"e4", parser.Square{File: 4, Rank: 3}, 36},
		{"e2", parser.Square{File: 4, Rank: 1}, 52},
		{"d5", parser.Square{File: 3, Rank: 4}, 27},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SquareIndex(tt.sq)
			if err != nil {
				t.Fatalf("SquareIndex(%+v) erro inesperado: %v", tt.sq, err)
			}
			if got != tt.want {
				t.Errorf("SquareIndex(%+v) = %d, esperado %d", tt.sq, got, tt.want)
			}
		})
	}
}

func TestSquareIndexForaDoTabuleiro(t *testing.T) {
	fora := []parser.Square{
		{File: -1, Rank: -1},
		{File: -1, Rank: 3},
		{File: 3, Rank: -1},
		{File: 8, Rank: 0},
		{File: 0, Rank: 8},
	}

	for _, sq := range fora {
		if idx, err := SquareIndex(sq); err == nil {
			t.Errorf("SquareIndex(%+v) deveria falhar, devolveu %d", sq, idx)
		}
	}
}

// IndexSquare tem de desfazer exatamente o que SquareIndex fez, nas 64 casas.
func TestIndexSquareEhInversaDeSquareIndex(t *testing.T) {
	for idx := 0; idx < BoardSize; idx++ {
		sq := IndexSquare(idx)
		volta, err := SquareIndex(sq)
		if err != nil {
			t.Fatalf("IndexSquare(%d) = %+v, que SquareIndex rejeitou: %v", idx, sq, err)
		}
		if volta != idx {
			t.Errorf("ida e volta do índice %d deu %d (casa %+v)", idx, volta, sq)
		}
	}
}

func TestIndexSquareForaDoTabuleiro(t *testing.T) {
	for _, idx := range []int{-1, BoardSize, 1000} {
		if sq := IndexSquare(idx); sq != (parser.Square{File: -1, Rank: -1}) {
			t.Errorf("IndexSquare(%d) = %+v, esperado {-1 -1}", idx, sq)
		}
	}
}

func TestSquareName(t *testing.T) {
	tests := map[int]string{0: "a8", 7: "h8", 36: "e4", 56: "a1", 63: "h1"}
	for idx, want := range tests {
		if got := SquareName(idx); got != want {
			t.Errorf("SquareName(%d) = %q, esperado %q", idx, got, want)
		}
	}
	if got := SquareName(-1); got != "-" {
		t.Errorf("SquareName(-1) = %q, esperado \"-\"", got)
	}
}

func TestParseSquareName(t *testing.T) {
	for idx := 0; idx < BoardSize; idx++ {
		name := SquareName(idx)
		got, err := ParseSquareName(name)
		if err != nil {
			t.Fatalf("ParseSquareName(%q) erro inesperado: %v", name, err)
		}
		if got != idx {
			t.Errorf("ParseSquareName(%q) = %d, esperado %d", name, got, idx)
		}
	}

	for _, name := range []string{"", "e", "e44", "i4", "e9", "e0"} {
		if got, err := ParseSquareName(name); err == nil {
			t.Errorf("ParseSquareName(%q) deveria falhar, devolveu %d", name, got)
		}
	}
}

// A direção do peão é contraintuitiva por causa da indexação: as brancas andam
// para índices menores.
func TestDirecaoDoPeao(t *testing.T) {
	e2, _ := ParseSquareName("e2")
	e4, _ := ParseSquareName("e4")
	if e2 != 52 || e4 != 36 {
		t.Fatalf("e2 = %d e e4 = %d, esperado 52 e 36", e2, e4)
	}
	if e4 >= e2 {
		t.Error("o peão branco tem de andar para índices menores")
	}

	e7, _ := ParseSquareName("e7")
	e5, _ := ParseSquareName("e5")
	if e5 <= e7 {
		t.Error("o peão preto tem de andar para índices maiores")
	}
}
