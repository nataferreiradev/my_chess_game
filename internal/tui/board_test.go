package tui

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	parser "github.com/nataferreiradev/chess_notation_parser"
	chess "github.com/nataferreiradev/my_chess_game"
)

// As peças dos dois lados são desenhadas com a mesma figura cheia de propósito
// (ver GamePiece.SolidSymbol), então é só a cor do texto que diz de quem é a
// peça. Se as duas saírem iguais, o tabuleiro fica ilegível.
func TestPecasDosDoisLadosSaemComCoresDiferentes(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	m := Model{styles: newStyles(renderer)}

	board := make(chess.Board, chess.BoardSize)

	board[0] = &chess.GamePiece{Piece: parser.Pawn, Color: chess.White}
	branca := m.renderSquare(board, 0, nil)

	board[0] = &chess.GamePiece{Piece: parser.Pawn, Color: chess.Black}
	preta := m.renderSquare(board, 0, nil)

	if branca == preta {
		t.Errorf("o peão branco saiu igual ao preto: %q", branca)
	}
	if !strings.Contains(branca, "\x1b[") {
		t.Errorf("o peão branco saiu sem cor nenhuma: %q", branca)
	}
}
