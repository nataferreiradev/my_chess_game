package my_chess_game

import parser "github.com/nataferreiradev/chess_notation_parser"

// BoardSize é o número de casas do tabuleiro.
const BoardSize = 64

// Board é o tabuleiro: BoardSize casas onde o índice 0 é a8 e o índice 63 é h1.
// Uma casa vazia é representada por nil.
type Board []*GamePiece

// NewBoard devolve um tabuleiro na posição inicial de uma partida.
func NewBoard() Board {
	b := make(Board, BoardSize)

	backRank := []parser.Piece{
		parser.Rook, parser.Knight, parser.Bishop, parser.Queen,
		parser.King, parser.Bishop, parser.Knight, parser.Rook,
	}

	for col, piece := range backRank {
		b[col] = &GamePiece{Piece: piece, Color: Black}
		b[56+col] = &GamePiece{Piece: piece, Color: White}
	}

	for col := 0; col < 8; col++ {
		b[8+col] = &GamePiece{Piece: parser.Pawn, Color: Black}
		b[48+col] = &GamePiece{Piece: parser.Pawn, Color: White}
	}

	return b
}
