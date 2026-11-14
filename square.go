package my_chess_game

import (
	"fmt"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

// As duas convenções de coordenada do projeto são invertidas, e essa é a fonte
// de bug mais provável de todo o motor:
//
//   - parser.Square tem Rank 0 = linha 1 e Rank 7 = linha 8.
//   - O índice do tabuleiro tem 0 = a8 (linha 8) e 63 = h1 (linha 1).
//
// Ou seja, a linha 8 fica no começo do slice e a linha 1 no fim. Toda conversão
// passa por SquareIndex e IndexSquare; nada no motor faz a conta na mão.

// row devolve a fileira do índice: 0 é a linha 8 e 7 é a linha 1.
func row(idx int) int { return idx / 8 }

// col devolve a coluna do índice: 0 é a coluna a e 7 é a coluna h.
func col(idx int) int { return idx % 8 }

// index monta o índice a partir da fileira e da coluna, ou -1 se cair fora.
func index(r, c int) int {
	if r < 0 || r > 7 || c < 0 || c > 7 {
		return -1
	}
	return r*8 + c
}

// onBoard diz se o índice está dentro do tabuleiro.
func onBoard(idx int) bool { return idx >= 0 && idx < BoardSize }

// SquareIndex converte uma casa do parser no índice do tabuleiro.
// Confira: a8 → 0, e4 → 36, h1 → 63.
func SquareIndex(sq parser.Square) (int, error) {
	if sq.File < 0 || sq.File > 7 || sq.Rank < 0 || sq.Rank > 7 {
		return -1, fmt.Errorf("casa fora do tabuleiro: %+v", sq)
	}
	return (7-sq.Rank)*8 + sq.File, nil
}

// IndexSquare converte um índice do tabuleiro na casa correspondente do parser.
// Devolve {-1, -1} se o índice estiver fora do tabuleiro.
func IndexSquare(idx int) parser.Square {
	if !onBoard(idx) {
		return parser.Square{File: -1, Rank: -1}
	}
	return parser.Square{File: col(idx), Rank: 7 - row(idx)}
}

// SquareName devolve o nome algébrico da casa, como "e4". Devolve "-" se o
// índice estiver fora do tabuleiro.
func SquareName(idx int) string {
	if !onBoard(idx) {
		return "-"
	}
	return fmt.Sprintf("%c%c", 'a'+col(idx), '8'-row(idx))
}

// ParseSquareName converte um nome algébrico como "e4" no índice do tabuleiro.
func ParseSquareName(name string) (int, error) {
	if len(name) != 2 {
		return -1, fmt.Errorf("casa inválida: %q", name)
	}
	c := int(name[0] - 'a')
	r := int('8' - name[1])
	idx := index(r, c)
	if idx < 0 {
		return -1, fmt.Errorf("casa inválida: %q", name)
	}
	return idx, nil
}
