package tui

import (
	"fmt"
	"strings"

	chess "github.com/nataferreiradev/my_chess_game"
	"github.com/nataferreiradev/my_chess_game/internal/room"
)

// renderBoard desenha o tabuleiro de casas alternadas, com as coordenadas nas
// bordas e as peças em Unicode.
//
// O tabuleiro sempre aparece do ponto de vista de quem está jogando: quem joga
// de pretas vê a linha 1 em cima. Espectador vê pelo lado das brancas.
func (m Model) renderBoard(snap room.Snapshot) string {
	flip := snap.YourColor == chess.Black && !snap.Spectator

	marked := markedSquares(snap.LastMove)

	var sb strings.Builder
	sb.WriteString(m.renderFiles(flip))
	sb.WriteByte('\n')

	for displayRow := 0; displayRow < 8; displayRow++ {
		r := displayRow
		if flip {
			r = 7 - displayRow
		}

		rank := fmt.Sprintf("%d ", 8-r)
		sb.WriteString(m.styles.coord.Render(rank))

		for displayCol := 0; displayCol < 8; displayCol++ {
			c := displayCol
			if flip {
				c = 7 - displayCol
			}
			sb.WriteString(m.renderSquare(snap.Board, r*8+c, marked))
		}

		sb.WriteString(m.styles.coord.Render(" " + fmt.Sprint(8-r)))
		sb.WriteByte('\n')
	}

	sb.WriteString(m.renderFiles(flip))
	return sb.String()
}

// renderFiles desenha a régua de colunas, na ordem em que o jogador as vê.
func (m Model) renderFiles(flip bool) string {
	var sb strings.Builder
	sb.WriteString("  ")

	for displayCol := 0; displayCol < 8; displayCol++ {
		c := displayCol
		if flip {
			c = 7 - displayCol
		}
		sb.WriteString(fmt.Sprintf(" %c ", 'a'+c))
	}

	return m.styles.coord.Render(sb.String())
}

// renderSquare desenha uma casa: o fundo alternado, e a peça colorida por lado.
func (m Model) renderSquare(board chess.Board, idx int, marked map[int]bool) string {
	dark := (idx/8+idx%8)%2 == 1

	var style = m.styles.lightSquare
	switch {
	case marked[idx] && dark:
		style = m.styles.darkMarked
	case marked[idx]:
		style = m.styles.lightMarked
	case dark:
		style = m.styles.darkSquare
	}

	piece := board[idx]
	if piece == nil {
		return style.Render("   ")
	}

	// Por padrão cada lado tem o seu desenho: as brancas vazadas (♔♕♖) e as
	// pretas cheias (♚♛♜). Assim os dois lados se distinguem pela figura, e não
	// só pela cor — que ainda é aplicada por cima, para o contraste.
	color, white := m.styles.blackPiece, piece.Color == chess.White
	if white {
		color = m.styles.whitePiece
	}

	return style.Foreground(color).Bold(white).Render(fmt.Sprintf(" %c ", m.symbolOf(*piece)))
}

// symbolOf escolhe a figura da peça conforme o modo de desenho da sessão.
//
// Terminal cuja fonte não tem as figuras vazadas as desenha com largura errada
// e desalinha o tabuleiro. Quem cair nisso troca para a figura cheia dos dois
// lados com o comando "figuras", separando-os só pela cor.
func (m Model) symbolOf(piece chess.GamePiece) rune {
	if m.solid {
		return piece.SolidSymbol()
	}
	return piece.Symbol()
}

// markedSquares devolve as casas que o último lance mexeu, para destacar.
func markedSquares(last *chess.AppliedMove) map[int]bool {
	if last == nil {
		return nil
	}

	marked := map[int]bool{last.From: true, last.To: true}
	if last.RookFrom >= 0 {
		marked[last.RookFrom] = true
		marked[last.RookTo] = true
	}
	if last.EnPassant && last.CapturedAt >= 0 {
		marked[last.CapturedAt] = true
	}

	return marked
}

// renderHistory monta a lista de lances em duas colunas, como numa planilha de
// partida: número, lance das brancas, lance das pretas.
func (m Model) renderHistory(history []chess.AppliedMove, limit int) string {
	if len(history) == 0 {
		return m.styles.label.Render("(nenhum lance ainda)")
	}

	var lines []string
	for i := 0; i < len(history); i += 2 {
		line := fmt.Sprintf("%2d. %-8s", i/2+1, history[i].Notation)
		if i+1 < len(history) {
			line += history[i+1].Notation
		}
		lines = append(lines, line)
	}

	// Só as últimas linhas cabem no painel; o resto rolou para fora.
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	return m.styles.value.Render(strings.Join(lines, "\n"))
}

// renderCaptured mostra as peças que cada lado já perdeu.
func (m Model) renderCaptured(history []chess.AppliedMove) string {
	var byWhite, byBlack []rune

	for _, applied := range history {
		if applied.Captured == nil {
			continue
		}
		if applied.Player == chess.White {
			byWhite = append(byWhite, m.symbolOf(*applied.Captured))
		} else {
			byBlack = append(byBlack, m.symbolOf(*applied.Captured))
		}
	}

	if len(byWhite) == 0 && len(byBlack) == 0 {
		return ""
	}

	return fmt.Sprintf("%s %s\n%s %s",
		m.styles.label.Render("brancas capturaram:"), string(byWhite),
		m.styles.label.Render("pretas capturaram: "), string(byBlack))
}
