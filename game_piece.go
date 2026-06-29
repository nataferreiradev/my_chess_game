package my_chess_game 

import parser "github.com/nataferreiradev/chess_notation_parser"

type Colors byte

const (
	Black Colors = 0
	White Colors = 1
)

type GamePiece struct{
	Piece parser.Piece
	Color Colors
}
