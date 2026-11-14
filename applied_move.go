package my_chess_game

import parser "github.com/nataferreiradev/chess_notation_parser"

// Outcome é a situação da partida depois de um lance.
type Outcome byte

const (
	// Ongoing é a partida ainda em andamento.
	Ongoing Outcome = iota
	// Checkmate é o xeque-mate: o rei da vez está atacado e não há lance legal.
	Checkmate
	// Stalemate é o afogamento: não há lance legal, mas o rei não está atacado.
	Stalemate
	// DrawFiftyMove é o empate pela regra dos 50 lances.
	DrawFiftyMove
	// DrawThreefold é o empate por repetição tripla da posição.
	DrawThreefold
	// DrawInsufficientMaterial é o empate por material insuficiente para matar.
	DrawInsufficientMaterial
)

// Over diz se a partida acabou.
func (o Outcome) Over() bool { return o != Ongoing }

// String descreve a situação em português, para exibição ao jogador.
func (o Outcome) String() string {
	switch o {
	case Checkmate:
		return "xeque-mate"
	case Stalemate:
		return "afogamento"
	case DrawFiftyMove:
		return "empate pela regra dos 50 lances"
	case DrawThreefold:
		return "empate por repetição tripla"
	case DrawInsufficientMaterial:
		return "empate por material insuficiente"
	}
	return "em andamento"
}

// AppliedMove descreve um lance já aplicado ao tabuleiro.
//
// É o que o árbitro transmite às sessões: um lance validado carrega tudo que as
// duas TUIs precisam para se atualizar sem reler a partida inteira, e tudo que
// o histórico precisa para ser reenviado a quem reconecta.
type AppliedMove struct {
	// Player é a cor de quem jogou.
	Player Color
	// Piece é o tipo de peça movida. Numa promoção é o peão, não a peça nova.
	Piece parser.Piece

	// From e To são as casas de origem e destino da peça movida.
	From, To int

	// Captured é a peça capturada, ou nil se não houve captura.
	Captured *GamePiece
	// CapturedAt é a casa de onde a peça capturada saiu. É igual a To em
	// qualquer captura normal, mas difere no en passant, onde o peão capturado
	// não está na casa de destino. Vale -1 se não houve captura.
	CapturedAt int
	// EnPassant diz se a captura foi en passant.
	EnPassant bool

	// Castle é o lado do roque, ou CastleNone. RookFrom e RookTo são o
	// deslocamento da torre no roque, e valem -1 fora dele.
	Castle           parser.CastleSide
	RookFrom, RookTo int

	// Promotion é a peça em que o peão virou, ou 0 se não houve promoção.
	Promotion parser.Piece

	// Check diz se o lance deixou o adversário em xeque.
	Check bool
	// Outcome é a situação da partida depois do lance.
	Outcome Outcome
	// Notation é a notação algébrica canônica do lance, já desambiguada e com
	// o sufixo de xeque ou mate. É ela que vai para o histórico.
	Notation string
}

// Move é um lance já resolvido: sem notação, sem ambiguidade, sem parse.
//
// É a moeda do caminho rápido — Moves gera, Apply aplica, Unapply desfaz — e
// também a representação interna que MakeMove usa depois de resolver a notação.
// Existe um gerador de lances só no projeto, e é este: um motor com o seu
// próprio tabuleiro acabaria jogando lances que as regras recusam.
//
// Move é um valor comparável e sem ponteiro de propósito. Guardar aqui um
// *GamePiece do tabuleiro vivo faria o lance apodrecer assim que a peça
// apontada mudasse de casa — e a busca guarda lances por muitos plies.
type Move struct {
	// From e To são as casas de origem e destino, no índice do tabuleiro
	// (0 = a8, 63 = h1).
	From, To int
	// Piece é a peça movida. Numa promoção é o peão, não a peça nova.
	Piece parser.Piece
	// Promotion é a peça em que o peão vira, ou 0 fora de uma promoção.
	Promotion parser.Piece

	// Captured é o tipo da peça capturada, ou 0 se não houve captura. A cor não
	// é guardada porque ela é sempre a do adversário de quem joga.
	Captured parser.Piece
	// CapturedAt é a casa de onde a peça capturada sai, ou -1 sem captura. É
	// igual a To em qualquer captura normal, e difere no en passant, onde o peão
	// capturado não está na casa de destino.
	CapturedAt int
	// EnPassant diz se a captura é en passant.
	EnPassant bool

	// Castle é o lado do roque, ou CastleNone. RookFrom e RookTo são o
	// deslocamento da torre, e valem -1 fora do roque.
	Castle           parser.CastleSide
	RookFrom, RookTo int
}

// IsCapture diz se o lance captura alguma peça, en passant inclusive.
func (m Move) IsCapture() bool { return m.CapturedAt >= 0 }
