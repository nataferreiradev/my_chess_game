package my_chess_game

import parser "github.com/nataferreiradev/chess_notation_parser"

type Board []GamePiece

func NewBoard() Board {
	b := make(Board, 64)

	for i := range b {
		b[i] = GamePiece{Piece: parser.Empty}
	}

	backRank := []parser.Piece{
		parser.Rook, parser.Knight, parser.Bishop, parser.Queen,
		parser.King, parser.Bishop, parser.Knight, parser.Rook,
	}

	for col, piece := range backRank {
		b[col] = GamePiece{Piece: piece, Color: Black}
		b[56+col] = GamePiece{Piece: piece, Color: White}
	}

	for col := 0; col < 8; col++ {
		b[8+col] = GamePiece{Piece: parser.Pawn, Color: Black}
		b[48+col] = GamePiece{Piece: parser.Pawn, Color: White}
	}

	return b
}
