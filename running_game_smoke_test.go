package my_chess_game

import (
	"testing"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

func TestMakeMoveSmoke(t *testing.T) {
	g := NewRunningGame()

	// 1. e4  (peão avança duas casas)
	if err := g.MakeMove("e4"); err != nil {
		t.Fatalf("e4: %v", err)
	}
	if got := g.board[squareToIndex(parser.Square{File: 4, Rank: 3})]; got.Piece != parser.Pawn || got.Color != White {
		t.Fatalf("e4 não posicionou o peão branco em e4: %+v", got)
	}

	// 1... e5
	if err := g.MakeMove("e5"); err != nil {
		t.Fatalf("e5: %v", err)
	}

	// 2. Nf3 (cavalo encontra a origem sozinho)
	if err := g.MakeMove("Nf3"); err != nil {
		t.Fatalf("Nf3: %v", err)
	}
	if got := g.board[squareToIndex(parser.Square{File: 5, Rank: 2})]; got.Piece != parser.Knight || got.Color != White {
		t.Fatalf("Nf3 não posicionou o cavalo: %+v", got)
	}

	// Lance ilegal: torre presa não pode ir a a4.
	if err := g.MakeMove("Ra4"); err == nil {
		t.Fatal("Ra4 deveria falhar (torre bloqueada)")
	}
}
