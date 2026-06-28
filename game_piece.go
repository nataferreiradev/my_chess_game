package my_chess_game 

import parser "github.com/nataferreiradev/chess_notation_parser"

type PieceColor byte

const (
	Black PieceColor = 0
	White PieceColor = 1
)

type GamePiece struct{
	Piece parser.Piece
	Color PieceColor
}
