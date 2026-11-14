package my_chess_game

import (
	"fmt"
	"strconv"
	"strings"

	parser "github.com/nataferreiradev/chess_notation_parser"
)

// InitialFEN é a posição inicial de uma partida em notação FEN.
const InitialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// A FEN descreve o tabuleiro da linha 8 para a linha 1, que é exatamente a
// ordem dos índices deste tabuleiro: o primeiro campo percorre 0..63 direto.

// FEN devolve a posição atual em notação FEN, com os seis campos.
func (game *RunningGame) FEN() string {
	return fmt.Sprintf("%s %d %d", game.positionKey(), game.halfmoveClock, game.fullmoveNumber)
}

// positionKey devolve os quatro primeiros campos da FEN: posição, cor da vez,
// direitos de roque e casa de en passant.
//
// É a chave da repetição tripla, e por isso deixa de fora os contadores de
// lance: duas posições se repetem mesmo que os relógios estejam diferentes.
func (game *RunningGame) positionKey() string {
	var sb strings.Builder

	for r := 0; r < 8; r++ {
		empty := 0
		for c := 0; c < 8; c++ {
			piece := game.board[index(r, c)]
			if piece == nil {
				empty++
				continue
			}
			if empty > 0 {
				sb.WriteString(strconv.Itoa(empty))
				empty = 0
			}
			sb.WriteByte(fenLetter(*piece))
		}
		if empty > 0 {
			sb.WriteString(strconv.Itoa(empty))
		}
		if r < 7 {
			sb.WriteByte('/')
		}
	}

	sb.WriteByte(' ')
	if game.playingColor == White {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('b')
	}

	sb.WriteByte(' ')
	sb.WriteString(game.castleField())

	sb.WriteByte(' ')
	if game.enPassant >= 0 {
		sb.WriteString(SquareName(game.enPassant))
	} else {
		sb.WriteByte('-')
	}

	return sb.String()
}

// castleField monta o campo de direitos de roque da FEN, no formato "KQkq".
func (game *RunningGame) castleField() string {
	var sb strings.Builder

	if game.castleRights[White].kingside {
		sb.WriteByte('K')
	}
	if game.castleRights[White].queenside {
		sb.WriteByte('Q')
	}
	if game.castleRights[Black].kingside {
		sb.WriteByte('k')
	}
	if game.castleRights[Black].queenside {
		sb.WriteByte('q')
	}

	if sb.Len() == 0 {
		return "-"
	}
	return sb.String()
}

// fenLetter devolve a letra FEN da peça: maiúscula para as brancas, minúscula
// para as pretas.
func fenLetter(piece GamePiece) byte {
	letter := byte(piece.Piece)
	if piece.Color == Black {
		return letter - 'A' + 'a'
	}
	return letter
}

// NewGameFromFEN monta uma partida a partir de uma posição em notação FEN.
//
// Os dois contadores finais são opcionais: sem eles a partida começa com o
// relógio dos 50 lances zerado e no lance 1.
func NewGameFromFEN(fen string) (*RunningGame, error) {
	fields := strings.Fields(fen)
	if len(fields) != 4 && len(fields) != 6 {
		return nil, fmt.Errorf("FEN precisa de 4 ou 6 campos, tem %d: %q", len(fields), fen)
	}

	board, err := parseFENPlacement(fields[0])
	if err != nil {
		return nil, err
	}

	var playing Color
	switch fields[1] {
	case "w":
		playing = White
	case "b":
		playing = Black
	default:
		return nil, fmt.Errorf("cor da vez inválida na FEN: %q", fields[1])
	}

	rights, err := parseFENCastling(fields[2])
	if err != nil {
		return nil, err
	}

	enPassant := -1
	if fields[3] != "-" {
		enPassant, err = ParseSquareName(fields[3])
		if err != nil {
			return nil, fmt.Errorf("casa de en passant inválida na FEN: %w", err)
		}
	}

	halfmove, fullmove := 0, 1
	if len(fields) == 6 {
		if halfmove, err = strconv.Atoi(fields[4]); err != nil || halfmove < 0 {
			return nil, fmt.Errorf("contador de 50 lances inválido na FEN: %q", fields[4])
		}
		if fullmove, err = strconv.Atoi(fields[5]); err != nil || fullmove < 1 {
			return nil, fmt.Errorf("número do lance inválido na FEN: %q", fields[5])
		}
	}

	game := &RunningGame{
		playingColor:   playing,
		board:          board,
		castleRights:   rights,
		enPassant:      enPassant,
		halfmoveClock:  halfmove,
		fullmoveNumber: fullmove,
		positions:      make(map[string]int),
	}

	game.positions[game.positionKey()] = 1

	// A posição carregada já pode ser mate, afogamento ou empate.
	check := game.InCheck()
	hasReply := len(game.legalMoves(playing)) > 0
	game.outcome = game.detectOutcome(check, hasReply)

	return game, nil
}

// parseFENPlacement lê o campo de posição da FEN, da linha 8 para a linha 1.
func parseFENPlacement(placement string) (Board, error) {
	rows := strings.Split(placement, "/")
	if len(rows) != 8 {
		return nil, fmt.Errorf("FEN precisa de 8 linhas, tem %d", len(rows))
	}

	board := make(Board, BoardSize)

	for r, spec := range rows {
		c := 0
		for i := 0; i < len(spec); i++ {
			char := spec[i]

			if char >= '1' && char <= '8' {
				c += int(char - '0')
				continue
			}

			piece, err := fenPiece(char)
			if err != nil {
				return nil, fmt.Errorf("linha %d da FEN: %w", 8-r, err)
			}
			if c > 7 {
				return nil, fmt.Errorf("linha %d da FEN tem casas demais", 8-r)
			}

			board[index(r, c)] = &piece
			c++
		}

		if c != 8 {
			return nil, fmt.Errorf("linha %d da FEN descreve %d casas, esperado 8", 8-r, c)
		}
	}

	return board, nil
}

// fenPiece converte a letra FEN de uma peça em GamePiece.
func fenPiece(char byte) (GamePiece, error) {
	color := White
	if char >= 'a' && char <= 'z' {
		color = Black
		char = char - 'a' + 'A'
	}

	switch parser.Piece(char) {
	case parser.Pawn, parser.Knight, parser.Bishop, parser.Rook, parser.Queen, parser.King:
		return GamePiece{Piece: parser.Piece(char), Color: color}, nil
	}

	return GamePiece{}, fmt.Errorf("peça desconhecida: %q", string(char))
}

// parseFENCastling lê o campo de direitos de roque da FEN.
func parseFENCastling(field string) ([2]castleRights, error) {
	var rights [2]castleRights
	if field == "-" {
		return rights, nil
	}

	for i := 0; i < len(field); i++ {
		switch field[i] {
		case 'K':
			rights[White].kingside = true
		case 'Q':
			rights[White].queenside = true
		case 'k':
			rights[Black].kingside = true
		case 'q':
			rights[Black].queenside = true
		default:
			return rights, fmt.Errorf("direito de roque inválido na FEN: %q", string(field[i]))
		}
	}

	return rights, nil
}
