package my_chess_game

import (
	"testing"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

func TestNewBoardTemTodasAsCasas(t *testing.T) {
	b := NewBoard()
	if len(b) != BoardSize {
		t.Fatalf("len(NewBoard()) = %d, esperado %d", len(b), BoardSize)
	}
}

func TestNewBoardPosicaoInicial(t *testing.T) {
	b := NewBoard()

	// Índice 0 é a8 e índice 63 é h1.
	casas := map[int]GamePiece{
		0:  {Piece: parser.Rook, Color: Black},
		4:  {Piece: parser.King, Color: Black},
		7:  {Piece: parser.Rook, Color: Black},
		56: {Piece: parser.Rook, Color: White},
		60: {Piece: parser.King, Color: White},
		63: {Piece: parser.Rook, Color: White},
	}

	for idx, esperado := range casas {
		if b[idx] == nil {
			t.Errorf("casa %d está vazia, esperado %+v", idx, esperado)
			continue
		}
		if *b[idx] != esperado {
			t.Errorf("casa %d = %+v, esperado %+v", idx, *b[idx], esperado)
		}
	}

	for idx := 8; idx < 16; idx++ {
		if b[idx] == nil || *b[idx] != (GamePiece{Piece: parser.Pawn, Color: Black}) {
			t.Errorf("casa %d deveria ter um peão preto, tem %v", idx, b[idx])
		}
	}

	for idx := 48; idx < 56; idx++ {
		if b[idx] == nil || *b[idx] != (GamePiece{Piece: parser.Pawn, Color: White}) {
			t.Errorf("casa %d deveria ter um peão branco, tem %v", idx, b[idx])
		}
	}

	for idx := 16; idx < 48; idx++ {
		if b[idx] != nil {
			t.Errorf("casa %d deveria estar vazia, tem %+v", idx, *b[idx])
		}
	}
}

func TestNewGameComecaComAsBrancas(t *testing.T) {
	game := NewGame()
	if game.CurrentPlayer() != White {
		t.Errorf("CurrentPlayer() = %v, esperado %v", game.CurrentPlayer(), White)
	}
}

func TestBoardEhCopiaProfunda(t *testing.T) {
	game := NewGame()

	copia := game.Board()
	copia[0] = nil
	copia[63].Color = Black

	if _, ok := game.PieceAt(0); !ok {
		t.Error("apagar uma casa da cópia esvaziou a casa da partida")
	}

	peca, ok := game.PieceAt(63)
	if !ok {
		t.Fatal("casa 63 deveria ter a torre branca")
	}
	if peca.Color != White {
		t.Errorf("alterar a peça da cópia mudou a partida: casa 63 = %v", peca.Color)
	}
}

func TestPieceAt(t *testing.T) {
	game := NewGame()

	if _, ok := game.PieceAt(-1); ok {
		t.Error("PieceAt(-1) deveria devolver false")
	}
	if _, ok := game.PieceAt(BoardSize); ok {
		t.Errorf("PieceAt(%d) deveria devolver false", BoardSize)
	}
	if _, ok := game.PieceAt(32); ok {
		t.Error("PieceAt(32) é uma casa vazia, deveria devolver false")
	}

	peca, ok := game.PieceAt(60)
	if !ok {
		t.Fatal("PieceAt(60) deveria devolver o rei branco")
	}
	if peca.Piece != parser.King || peca.Color != White {
		t.Errorf("PieceAt(60) = %+v, esperado rei branco", peca)
	}
}

func TestMakeMoveNotacaoInvalida(t *testing.T) {
	game := NewGame()

	for _, cmd := range []string{"", "e", "i4", "e9"} {
		if _, err := game.MakeMove(cmd); err == nil {
			t.Errorf("MakeMove(%q) deveria devolver erro", cmd)
		}
	}

	if game.CurrentPlayer() != White {
		t.Error("um lance inválido não deveria passar a vez")
	}
}

func TestMakeMoveAlternaAVez(t *testing.T) {
	game := NewGame()

	if _, err := game.MakeMove("e4"); err != nil {
		t.Fatalf("MakeMove(\"e4\") erro inesperado: %v", err)
	}
	if game.CurrentPlayer() != Black {
		t.Errorf("depois de um lance a vez deveria ser das pretas, é das %v", game.CurrentPlayer())
	}

	if _, err := game.MakeMove("e5"); err != nil {
		t.Fatalf("MakeMove(\"e5\") erro inesperado: %v", err)
	}
	if game.CurrentPlayer() != White {
		t.Errorf("depois de dois lances a vez deveria voltar às brancas, é das %v", game.CurrentPlayer())
	}
}

func TestColorString(t *testing.T) {
	if White.String() != "brancas" {
		t.Errorf("White.String() = %q, esperado \"brancas\"", White.String())
	}
	if Black.String() != "pretas" {
		t.Errorf("Black.String() = %q, esperado \"pretas\"", Black.String())
	}
}
