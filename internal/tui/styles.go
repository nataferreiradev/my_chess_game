package tui

import "github.com/charmbracelet/lipgloss"

// styles junta tudo que a TUI usa para desenhar.
//
// Os estilos nascem do renderer da sessão, e não de um renderer global: o
// perfil de cor tem de ser o do terminal de quem está jogando, e não o do
// servidor. O servidor só impõe um mínimo de 256 cores (ver newServer), porque
// abaixo disso as duas cores de peça caem na mesma cor do terminal e o
// tabuleiro fica ilegível.
type styles struct {
	lightSquare lipgloss.Style
	darkSquare  lipgloss.Style
	lightMarked lipgloss.Style
	darkMarked  lipgloss.Style

	whitePiece lipgloss.Color
	blackPiece lipgloss.Color

	coord    lipgloss.Style
	title    lipgloss.Style
	selected lipgloss.Style
	panel    lipgloss.Style
	label    lipgloss.Style
	value    lipgloss.Style
	help     lipgloss.Style
	notice   lipgloss.Style
	failure  lipgloss.Style
	win      lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) styles {
	return styles{
		lightSquare: r.NewStyle().Background(lipgloss.Color("#D9C2A3")),
		darkSquare:  r.NewStyle().Background(lipgloss.Color("#9C7A56")),
		// As casas do último lance ficam esverdeadas, para o adversário achar
		// de relance o que mudou.
		lightMarked: r.NewStyle().Background(lipgloss.Color("#CBD98A")),
		darkMarked:  r.NewStyle().Background(lipgloss.Color("#94A85C")),

		// As duas cores têm de ser bem distantes uma da outra e das duas cores
		// de casa: as peças dos dois lados usam a mesma figura cheia, então é
		// só a cor do texto que diz de quem é a peça.
		whitePiece: lipgloss.Color("#FFFFFF"),
		blackPiece: lipgloss.Color("#101014"),

		coord:    r.NewStyle().Foreground(lipgloss.Color("245")),
		title:    r.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		selected: r.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		panel:    r.NewStyle().MarginLeft(3),
		label:    r.NewStyle().Foreground(lipgloss.Color("245")),
		value:    r.NewStyle().Foreground(lipgloss.Color("252")),
		help:     r.NewStyle().Foreground(lipgloss.Color("241")),
		notice:   r.NewStyle().Foreground(lipgloss.Color("214")),
		failure:  r.NewStyle().Foreground(lipgloss.Color("203")),
		win:      r.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
	}
}
