package my_chess_game

import (
	"strings"
	"testing"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

func TestFENDaPosicaoInicial(t *testing.T) {
	if got := NewGame().FEN(); got != InitialFEN {
		t.Errorf("FEN() = %q, esperado %q", got, InitialFEN)
	}
}

func TestFENIdaEVolta(t *testing.T) {
	fens := []string{
		InitialFEN,
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"4k3/8/8/8/8/8/8/4K3 b - - 13 42",
		"rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR w KQkq c6 0 2",
	}

	for _, fen := range fens {
		t.Run(fen, func(t *testing.T) {
			game, err := NewGameFromFEN(fen)
			if err != nil {
				t.Fatalf("NewGameFromFEN(%q) erro inesperado: %v", fen, err)
			}
			if got := game.FEN(); got != fen {
				t.Errorf("ida e volta mudou a FEN:\n  entrou: %s\n  saiu:   %s", fen, got)
			}
		})
	}
}

func TestFENSemContadores(t *testing.T) {
	game, err := NewGameFromFEN("4k3/8/8/8/8/8/8/4K3 w - -")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := game.FEN(); got != "4k3/8/8/8/8/8/8/4K3 w - - 0 1" {
		t.Errorf("FEN() = %q, os contadores deveriam começar em 0 e 1", got)
	}
}

func TestFENLeAPosicaoCerta(t *testing.T) {
	// A FEN descreve da linha 8 para a linha 1, que é a ordem dos índices.
	game := mustGame(t, InitialFEN)

	assertPiece(t, game, "a8", parser.Rook, Black)
	assertPiece(t, game, "e8", parser.King, Black)
	assertPiece(t, game, "e1", parser.King, White)
	assertPiece(t, game, "h1", parser.Rook, White)
	assertEmpty(t, game, "e4", "d5")

	if game.CurrentPlayer() != White {
		t.Error("a FEN inicial põe as brancas na vez")
	}
}

func TestFENInvalida(t *testing.T) {
	tests := []struct {
		name string
		fen  string
	}{
		{"vazia", ""},
		{"campos de menos", "4k3/8/8/8/8/8/8/4K3 w"},
		{"campos de mais", "4k3/8/8/8/8/8/8/4K3 w - - 0 1 2 3"},
		{"linhas de menos", "4k3/8/8/8/8/8/4K3 w - - 0 1"},
		{"linha curta", "4k3/8/8/8/8/8/8/4K2 w - - 0 1"},
		{"linha longa", "4k3/8/8/8/8/8/8/4K3PPP w - - 0 1"},
		{"peça desconhecida", "4k3/8/8/8/8/8/8/4X3 w - - 0 1"},
		{"cor inválida", "4k3/8/8/8/8/8/8/4K3 x - - 0 1"},
		{"roque inválido", "4k3/8/8/8/8/8/8/4K3 w XY - 0 1"},
		{"en passant inválido", "4k3/8/8/8/8/8/8/4K3 w - z9 0 1"},
		{"contador negativo", "4k3/8/8/8/8/8/8/4K3 w - - -1 1"},
		{"lance zero", "4k3/8/8/8/8/8/8/4K3 w - - 0 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if game, err := NewGameFromFEN(tt.fen); err == nil {
				t.Errorf("NewGameFromFEN(%q) deveria falhar, devolveu %+v", tt.fen, game)
			}
		})
	}
}

func TestFENAcompanhaOsDireitosDeRoque(t *testing.T) {
	game := mustGame(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")

	mustPlay(t, game, "Ke2")
	if !strings.Contains(game.FEN(), " kq ") {
		t.Errorf("o rei branco se moveu: as brancas perdem os dois direitos. FEN = %q", game.FEN())
	}

	mustPlay(t, game, "Rh7")
	if !strings.Contains(game.FEN(), " q ") {
		t.Errorf("a torre de h8 se moveu: as pretas perdem o roque pequeno. FEN = %q", game.FEN())
	}
}

func TestFENDetectaFimDeJogoNaPosicaoCarregada(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		want Outcome
	}{
		{"mate já na posição", "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3", Checkmate},
		{"afogamento já na posição", "7k/5Q2/8/8/8/8/8/K7 b - - 0 1", Stalemate},
		{"material insuficiente", "4k3/8/8/8/8/8/8/4K3 w - - 0 1", DrawInsufficientMaterial},
		{"partida normal", InitialFEN, Ongoing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := mustGame(t, tt.fen)
			if got := game.Outcome(); got != tt.want {
				t.Errorf("Outcome() = %v, esperado %v", got, tt.want)
			}
		})
	}
}

func TestSymbol(t *testing.T) {
	tests := []struct {
		piece GamePiece
		want  rune
	}{
		{GamePiece{Piece: parser.King, Color: White}, '♔'},
		{GamePiece{Piece: parser.Pawn, Color: White}, '♙'},
		{GamePiece{Piece: parser.King, Color: Black}, '♚'},
		{GamePiece{Piece: parser.Rook, Color: Black}, '♜'},
	}

	for _, tt := range tests {
		if got := tt.piece.Symbol(); got != tt.want {
			t.Errorf("%+v.Symbol() = %q, esperado %q", tt.piece, got, tt.want)
		}
	}

	if got := (GamePiece{Piece: 'Z'}).Symbol(); got != ' ' {
		t.Errorf("peça desconhecida deveria virar espaço, deu %q", got)
	}
}

func TestLegalMoves(t *testing.T) {
	game := NewGame()

	moves := game.LegalMoves()
	if len(moves) != 20 {
		t.Errorf("a posição inicial tem 20 lances legais, LegalMoves() deu %d", len(moves))
	}

	found := make(map[string]bool, len(moves))
	for _, move := range moves {
		found[move] = true
	}
	for _, want := range []string{"e4", "e3", "Nf3", "Nh3", "a4", "h3"} {
		if !found[want] {
			t.Errorf("%q deveria estar entre os lances legais", want)
		}
	}

	// Numa partida encerrada não há lance legal nenhum.
	mate := mustGame(t, "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3")
	if got := mate.LegalMoves(); got != nil {
		t.Errorf("LegalMoves() numa partida encerrada = %v, esperado nil", got)
	}
}
