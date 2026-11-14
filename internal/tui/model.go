// Package tui é a interface de texto que cada sessão SSH enxerga.
//
// Uma instância por sessão. A TUI é de propósito burra: ela não sabe as regras
// do xadrez e nunca valida um lance. Ela manda o texto que o jogador digitou
// para a sala e desenha o snapshot que recebe de volta. Quem sabe as regras é o
// package my_chess_game, e quem manda na partida é o árbitro da sala.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	chess "github.com/nataferreiradev/my_chess_game"
	"github.com/nataferreiradev/my_chess_game/internal/lobby"
	"github.com/nataferreiradev/my_chess_game/internal/room"
)

// historyLines é quantos lances do histórico cabem no painel lateral.
const historyLines = 16

// refreshEvery é de quanto em quanto tempo a lista de partidas se atualiza
// sozinha, para uma sala criada agora aparecer para quem já está olhando.
const refreshEvery = 2 * time.Second

// screen é a tela em que a sessão está.
type screen int

const (
	screenName   screen = iota // "como você quer ser chamado?"
	screenHome                 // o menu: criar sala, buscar partidas
	screenCreate               // o formulário da sala nova: título e senha
	screenBrowse               // a lista de salas abertas
	screenUnlock               // "essa sala pede senha"
	screenGame                 // o tabuleiro
)

// Seat é o lugar da sessão no servidor: a sala em que ela está.
//
// Ele existe fora do Model porque a limpeza precisa acontecer mesmo quando a
// sessão cai sem passar pela TUI — um ssh morto no meio da partida tem de
// avisar a sala do mesmo jeito.
type Seat struct {
	mu     sync.Mutex
	lobby  *lobby.Lobby
	id     room.PlayerID
	name   string
	room   *room.Room
	prog   *tea.Program
	closed bool
}

// Attach liga o Seat ao programa bubbletea da sessão. Tem de ser chamado antes
// de o programa rodar.
func (s *Seat) Attach(p *tea.Program) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prog = p
}

// send entrega uma mensagem à TUI. É chamado pela goroutine árbitro, então só
// repassa para o bubbletea, que é seguro de chamar de qualquer goroutine.
func (s *Seat) send(msg tea.Msg) {
	s.mu.Lock()
	prog, closed := s.prog, s.closed
	s.mu.Unlock()

	if prog != nil && !closed {
		prog.Send(msg)
	}
}

// Close solta o lugar: avisa a sala que o jogador caiu. Pode ser chamado mais
// de uma vez.
func (s *Seat) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	r := s.room
	s.mu.Unlock()

	if r != nil {
		r.Leave(s.id)
	}
}

func (s *Seat) setRoom(r *room.Room) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.room = r
}

func (s *Seat) setName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
}

func (s *Seat) playerName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// Model é a TUI de uma sessão.
type Model struct {
	styles styles
	seat   *Seat

	screen screen
	name   textinput.Model
	move   textinput.Model

	// title e secret são o formulário da sala nova, e field diz qual dos dois
	// está em foco. unlock é a senha digitada para entrar numa sala trancada.
	title  textinput.Model
	secret textinput.Model
	field  int
	unlock textinput.Model

	// menu é a opção escolhida no home, e rooms/cursor são a lista de partidas
	// e a linha selecionada nela.
	menu   int
	rooms  []lobby.Open
	cursor int

	snap      *room.Snapshot
	notice    string
	noticeErr bool

	// solid desenha os dois lados com a figura cheia (♚♛♜), separados só pela
	// cor do texto. É a saída para o terminal cuja fonte não tem as figuras
	// vazadas das brancas e desalinha o tabuleiro ao tentar desenhá-las.
	solid bool

	width, height int
	quitting      bool
}

// menuItems são as opções do home, na ordem em que aparecem.
var menuItems = []string{"criar sala", "buscar partidas", "mudar nome", "sair"}

// mensagens internas da TUI
type (
	joinedMsg  struct{ room *room.Room }
	roomsMsg   struct{ rooms []lobby.Open }
	refreshMsg struct{}
	failMsg    struct {
		text string
		// wrongPass distingue a senha errada dos outros erros: só ela deixa a
		// sessão na tela da senha, para o jogador tentar de novo.
		wrongPass bool
	}
	updateMsg struct{ update room.Update }
)

// New monta a TUI de uma sessão.
//
// ctx é o contexto da sessão SSH: quando ele termina — o jogador fechou o
// terminal, a conexão caiu — o lugar é liberado sozinho. O renderer vem da
// sessão para as cores serem as do terminal do cliente.
func New(ctx context.Context, l *lobby.Lobby, id room.PlayerID, name string, renderer *lipgloss.Renderer) (Model, *Seat) {
	move := textinput.New()
	move.Placeholder = "seu lance (e4, Nf3, exd5, O-O, e8=Q)"
	move.Prompt = "> "
	move.CharLimit = 16
	move.Width = 40

	nick := textinput.New()
	nick.Placeholder = "seu nome"
	nick.Prompt = "> "
	nick.CharLimit = 20
	nick.Width = 40
	nick.SetValue(name)
	nick.CursorEnd()
	nick.Focus()

	title := textinput.New()
	title.Placeholder = "nome da sala (opcional)"
	title.Prompt = "> "
	title.CharLimit = 30
	title.Width = 40

	// A senha aparece como pontinhos: o jogador costuma estar num terminal que
	// alguém mais pode estar vendo, e ela não vale só para esta sala — é comum
	// reusarem senha.
	secret := newSecretInput("senha (vazio = sala aberta)")
	unlock := newSecretInput("senha da sala")

	seat := &Seat{lobby: l, id: id, name: name}

	go func() {
		<-ctx.Done()
		seat.Close()
	}()

	return Model{
		styles: newStyles(renderer),
		seat:   seat,
		screen: screenName,
		name:   nick,
		move:   move,
		title:  title,
		secret: secret,
		unlock: unlock,
	}, seat
}

// newSecretInput monta um campo de senha, escondida atrás de pontinhos.
func newSecretInput(placeholder string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Prompt = "> "
	in.CharLimit = 32
	in.Width = 40
	in.EchoMode = textinput.EchoPassword
	in.EchoCharacter = '•'
	return in
}

// Init abre a sessão na tela do nome — ou direto no tabuleiro, se o jogador já
// tinha uma partida em andamento. É o caminho da reconexão.
func (m Model) Init() tea.Cmd {
	if r := m.seat.lobby.Current(m.seat.id); r != nil {
		return tea.Batch(textinput.Blink, func() tea.Msg { return joinedMsg{room: r} })
	}
	return textinput.Blink
}

// createRoom abre uma sala nova, com o título e a senha do formulário, e entra
// nela. Título vazio vira o nome padrão, senha vazia deixa a sala aberta.
func (m Model) createRoom(title, password string) tea.Cmd {
	return func() tea.Msg {
		return joinedMsg{room: m.seat.lobby.Create(m.seat.id, m.seat.playerName(), title, password)}
	}
}

// joinRoom entra numa sala da lista: jogando, se a cadeira estiver vazia, ou
// assistindo, se a partida já tiver dois.
func (m Model) joinRoom(code, password string) tea.Cmd {
	return func() tea.Msg {
		r, err := m.seat.lobby.Join(code, m.seat.id, m.seat.playerName(), password)
		if err != nil {
			return failMsg{text: err.Error(), wrongPass: errors.Is(err, lobby.ErrWrongPassword)}
		}
		return joinedMsg{room: r}
	}
}

// listRooms busca as salas abertas para a tela de procura.
func (m Model) listRooms() tea.Msg {
	return roomsMsg{rooms: m.seat.lobby.List()}
}

// refreshTick reagenda a atualização da lista.
func refreshTick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return refreshMsg{} })
}

// listen liga a sala à TUI: cada atualização vira uma mensagem do bubbletea.
func (m Model) listen(r *room.Room) tea.Cmd {
	return func() tea.Msg {
		seat := m.seat
		r.Join(seat.id, seat.name, func(u room.Update) {
			seat.send(updateMsg{update: u})
		})
		return nil
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case joinedMsg:
		m.seat.setRoom(msg.room)
		m.unlock.SetValue("")
		m.unlock.Blur()
		m.screen = screenGame
		m.notice, m.noticeErr = "", false
		m.move.Focus()
		return m, m.listen(msg.room)

	case roomsMsg:
		m.rooms = msg.rooms
		if m.cursor >= len(m.rooms) {
			m.cursor = max(0, len(m.rooms)-1)
		}
		return m, nil

	case refreshMsg:
		if m.screen != screenBrowse {
			return m, nil // saiu da lista: para de atualizar
		}
		return m, tea.Batch(m.listRooms, refreshTick())

	case failMsg:
		m.notice, m.noticeErr = msg.text, true
		if m.screen == screenUnlock && !msg.wrongPass {
			// A sala sumiu ou o jogador já está em outra: insistir na senha não
			// resolve, então ele volta para a lista.
			m.screen = screenBrowse
			m.unlock.Blur()
		}
		return m, m.listRooms

	case updateMsg:
		return m.handleUpdate(msg.update)
	}

	// O que sobra é do bubbles — o piscar do cursor, por exemplo — e vai para o
	// campo que está em foco.
	var cmd tea.Cmd
	switch m.screen {
	case screenName:
		m.name, cmd = m.name.Update(msg)
	case screenCreate:
		m.title, m.secret, cmd = updateForm(m.title, m.secret, m.field, msg)
	case screenUnlock:
		m.unlock, cmd = m.unlock.Update(msg)
	default:
		m.move, cmd = m.move.Update(msg)
	}
	return m, cmd
}

// updateForm entrega a mensagem ao campo do formulário que está em foco.
func updateForm(title, secret textinput.Model, field int, msg tea.Msg) (textinput.Model, textinput.Model, tea.Cmd) {
	var cmd tea.Cmd
	if field == 0 {
		title, cmd = title.Update(msg)
	} else {
		secret, cmd = secret.Update(msg)
	}
	return title, secret, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m.quit()
	}

	switch m.screen {
	case screenName:
		return m.keyName(msg)
	case screenHome:
		return m.keyHome(msg)
	case screenCreate:
		return m.keyCreate(msg)
	case screenUnlock:
		return m.keyUnlock(msg)
	case screenBrowse:
		return m.keyBrowse(msg)
	}
	return m.keyGame(msg)
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	m.seat.Close()
	return m, tea.Quit
}

// keyName cuida da primeira tela: o jogador escolhe como quer ser chamado.
func (m Model) keyName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		name := strings.TrimSpace(m.name.Value())
		if name == "" {
			m.notice, m.noticeErr = "escolha um nome para os outros te reconhecerem.", true
			return m, nil
		}

		m.seat.setName(name)
		m.name.Blur()
		m.screen, m.menu = screenHome, 0
		m.notice, m.noticeErr = "", false
		return m, nil
	}

	var cmd tea.Cmd
	m.name, cmd = m.name.Update(msg)
	return m, cmd
}

// keyHome cuida do menu. As opções respondem às setas, ao j/k e ao número da
// linha — quem prefere digitar não precisa navegar.
func (m Model) keyHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "up", "k":
		m.menu = (m.menu - 1 + len(menuItems)) % len(menuItems)
	case "down", "j", "tab":
		m.menu = (m.menu + 1) % len(menuItems)
	case "1", "2", "3", "4":
		m.menu = int(key[0] - '1')
		return m.chooseMenu()
	case "enter":
		return m.chooseMenu()
	case "q", "esc":
		return m.quit()
	}
	return m, nil
}

// chooseMenu executa a opção do menu que está selecionada.
func (m Model) chooseMenu() (tea.Model, tea.Cmd) {
	m.notice, m.noticeErr = "", false

	switch m.menu {
	case 0:
		return m.openCreate()
	case 1:
		m.screen, m.cursor = screenBrowse, 0
		return m, tea.Batch(m.listRooms, refreshTick())
	case 2:
		m.screen = screenName
		m.name.Focus()
		m.name.CursorEnd()
		return m, textinput.Blink
	}
	return m.quit()
}

// openCreate abre o formulário da sala nova, com os campos limpos.
func (m Model) openCreate() (tea.Model, tea.Cmd) {
	m.screen, m.field = screenCreate, 0
	m.title.SetValue("")
	m.secret.SetValue("")
	m.title.Focus()
	m.secret.Blur()
	m.notice, m.noticeErr = "", false
	return m, textinput.Blink
}

// keyCreate cuida do formulário da sala nova: título em cima, senha embaixo.
func (m Model) keyCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen, m.menu = screenHome, 0
		m.title.Blur()
		m.secret.Blur()
		return m, nil

	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		m.field = 1 - m.field
		return m.focusForm()

	case tea.KeyEnter:
		// Enter no título desce para a senha; na senha, cria a sala. Assim
		// ninguém abre sem querer uma sala aberta ao apertar enter duas vezes.
		if m.field == 0 {
			m.field = 1
			return m.focusForm()
		}

		title, password := strings.TrimSpace(m.title.Value()), m.secret.Value()
		m.title.Blur()
		m.secret.Blur()
		return m, m.createRoom(title, password)
	}

	var cmd tea.Cmd
	m.title, m.secret, cmd = updateForm(m.title, m.secret, m.field, msg)
	return m, cmd
}

// focusForm põe o cursor no campo selecionado do formulário.
func (m Model) focusForm() (tea.Model, tea.Cmd) {
	if m.field == 0 {
		m.secret.Blur()
		m.title.Focus()
		m.title.CursorEnd()
	} else {
		m.title.Blur()
		m.secret.Focus()
		m.secret.CursorEnd()
	}
	return m, textinput.Blink
}

// keyUnlock cuida da tela de senha de uma sala trancada.
func (m Model) keyUnlock(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenBrowse
		m.unlock.Blur()
		m.notice, m.noticeErr = "", false
		return m, tea.Batch(m.listRooms, refreshTick())

	case tea.KeyEnter:
		if m.cursor >= len(m.rooms) {
			m.screen = screenBrowse
			return m, m.listRooms
		}

		password := m.unlock.Value()
		m.unlock.SetValue("")
		return m, m.joinRoom(m.rooms[m.cursor].Code, password)
	}

	var cmd tea.Cmd
	m.unlock, cmd = m.unlock.Update(msg)
	return m, cmd
}

// keyBrowse cuida da lista de partidas abertas.
func (m Model) keyBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rooms)-1 {
			m.cursor++
		}
	case "r":
		return m, m.listRooms
	case "c":
		return m.openCreate()
	case "esc", "q", "b":
		m.screen = screenHome
		m.notice, m.noticeErr = "", false
	case "enter":
		return m.enterSelected()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if n := int(key[0] - '1'); n < len(m.rooms) {
			m.cursor = n
			return m.enterSelected()
		}
	}
	return m, nil
}

// enterSelected entra na sala em que o cursor está.
func (m Model) enterSelected() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.rooms) {
		m.notice, m.noticeErr = "nenhuma sala aberta: aperte c para criar a sua.", true
		return m, nil
	}

	if m.rooms[m.cursor].Locked {
		m.screen = screenUnlock
		m.unlock.SetValue("")
		m.unlock.Focus()
		m.notice, m.noticeErr = "", false
		return m, textinput.Blink
	}

	return m, m.joinRoom(m.rooms[m.cursor].Code, "")
}

// keyGame cuida da tela do tabuleiro: tudo que não é atalho vira texto do
// campo de lance.
func (m Model) keyGame(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		text := strings.TrimSpace(m.move.Value())
		m.move.SetValue("")
		return m.submit(text)
	}

	var cmd tea.Cmd
	m.move, cmd = m.move.Update(msg)
	return m, cmd
}

// submit interpreta o que o jogador digitou: ou um comando da sessão, ou um
// lance, que vai cru para a sala.
func (m Model) submit(text string) (tea.Model, tea.Cmd) {
	if text == "" {
		return m, nil
	}

	switch strings.ToLower(strings.TrimPrefix(text, "/")) {
	case "sair", "quit", "exit":
		return m.quit()

	case "menu", "voltar":
		// Sair da partida é só largar a sala: ela continua de pé, e entrar de
		// novo no mesmo código volta para onde parou.
		if r := m.currentRoom(); r != nil {
			r.Leave(m.seat.id)
		}
		m.seat.setRoom(nil)
		m.snap = nil
		m.move.Blur()
		m.screen, m.menu = screenHome, 0
		m.notice, m.noticeErr = "", false
		return m, nil

	case "figuras", "simbolos", "símbolos":
		m.solid = !m.solid
		m.notice = "os dois lados agora usam a figura cheia (♚♛♜), separados pela cor. " +
			"digite \"figuras\" de novo para voltar."
		if !m.solid {
			m.notice = "as brancas voltaram às figuras vazadas (♔♕♖)."
		}
		m.noticeErr = false
		return m, nil

	case "desistir", "resign":
		if r := m.currentRoom(); r != nil {
			r.Resign(m.seat.id)
		}
		return m, nil

	case "ajuda", "help", "?":
		m.notice = "digite o lance em notação algébrica: e4, Nf3, exd5, O-O, e8=Q, Bxe4+. " +
			"\"desistir\" entrega a partida, \"menu\" volta para a tela inicial, " +
			"\"figuras\" troca o desenho das peças, \"sair\" fecha a sessão."
		m.noticeErr = false
		return m, nil
	}

	r := m.currentRoom()
	if r == nil {
		m.notice = "ainda não há partida: volte ao menu e crie ou escolha uma sala."
		m.noticeErr = true
		return m, nil
	}

	// O lance vai como texto para a sala. Quem parseia e valida é o servidor;
	// a TUI não sabe as regras.
	r.Move(m.seat.id, text)
	return m, nil
}

func (m Model) handleUpdate(update room.Update) (tea.Model, tea.Cmd) {
	switch u := update.(type) {
	case room.Snapshot:
		m.snap = &u
		// Um snapshot novo significa que alguma coisa aconteceu: o erro do
		// lance anterior já não vale.
		if m.noticeErr {
			m.notice, m.noticeErr = "", false
		}

	case room.Notice:
		m.notice, m.noticeErr = u.Text, u.Error
	}

	return m, nil
}

func (m Model) currentRoom() *room.Room {
	m.seat.mu.Lock()
	defer m.seat.mu.Unlock()
	return m.seat.room
}

func (m Model) View() string {
	if m.quitting {
		return "até a próxima!\n"
	}

	var parts []string
	switch m.screen {
	case screenName:
		parts = m.nameView()
	case screenHome:
		parts = m.homeView()
	case screenCreate:
		parts = m.createView()
	case screenUnlock:
		parts = m.unlockView()
	case screenBrowse:
		parts = m.browseView()
	default:
		parts = m.gameView()
	}

	head := []string{m.styles.title.Render("xadrez por ssh"), ""}
	parts = append(head, parts...)

	if m.notice != "" {
		style := m.styles.notice
		if m.noticeErr {
			style = m.styles.failure
		}
		parts = append(parts, "", style.Render(m.notice))
	}

	return strings.Join(parts, "\n") + "\n"
}

// nameView é a porta de entrada: só o nome.
func (m Model) nameView() []string {
	return []string{
		m.styles.value.Render("como você quer ser chamado?"),
		m.styles.label.Render("é o nome que aparece para o adversário."),
		"",
		m.name.View(),
		"",
		m.styles.help.Render("enter confirma · ctrl+c sai"),
	}
}

// homeView é o menu inicial.
func (m Model) homeView() []string {
	lines := []string{
		m.styles.label.Render("jogador: ") + m.styles.value.Render(m.seat.playerName()),
		"",
	}

	for i, item := range menuItems {
		line := fmt.Sprintf("  %d. %s", i+1, item)
		if i == m.menu {
			lines = append(lines, m.styles.selected.Render(fmt.Sprintf("› %d. %s", i+1, item)))
			continue
		}
		lines = append(lines, m.styles.value.Render(line))
	}

	return append(lines,
		"",
		m.styles.help.Render("↑/↓ escolhe · enter confirma · ou aperte o número"),
	)
}

// createView é o formulário da sala nova.
func (m Model) createView() []string {
	name, secret := m.styles.value, m.styles.value
	if m.field == 0 {
		name = m.styles.selected
	} else {
		secret = m.styles.selected
	}

	return []string{
		m.styles.value.Render("sala nova"),
		m.styles.label.Render("com senha, a sala aparece na lista mas só entra quem souber a senha."),
		"",
		name.Render("nome da sala"),
		m.title.View(),
		"",
		secret.Render("senha"),
		m.secret.View(),
		"",
		m.styles.help.Render("tab troca de campo · enter na senha cria a sala · esc volta"),
	}
}

// unlockView pede a senha da sala trancada que o jogador escolheu.
func (m Model) unlockView() []string {
	var sala string
	if m.cursor < len(m.rooms) {
		open := m.rooms[m.cursor]
		sala = fmt.Sprintf("#%s %s · %s", open.Code, open.Title, open.Host)
	}

	return []string{
		m.styles.value.Render("essa sala pede senha"),
		m.styles.label.Render(sala),
		"",
		m.unlock.View(),
		"",
		m.styles.help.Render("enter entra · esc volta para a lista"),
	}
}

// browseView é a lista de partidas abertas: as que esperam adversário primeiro,
// as cheias depois, que dá para assistir.
func (m Model) browseView() []string {
	lines := []string{
		m.styles.value.Render("partidas abertas"),
		"",
	}

	if len(m.rooms) == 0 {
		lines = append(lines, m.styles.label.Render("nenhuma sala aberta no momento."))
	}

	for i, open := range m.rooms {
		who := open.Host + " · " + m.styles.win.Render("esperando adversário")
		if !open.Waiting {
			who = fmt.Sprintf("%s x %s · %s", open.Host, open.Guest,
				m.styles.label.Render("em jogo, dá para assistir"))
		}

		// O cadeado avisa que a sala pede senha antes de o jogador entrar nela.
		lock := " "
		if open.Locked {
			lock = "🔒"
		}

		head := fmt.Sprintf("%d. #%-3s %s %-22s ", i+1, open.Code, lock, truncate(open.Title, 22))
		if i == m.cursor {
			lines = append(lines, m.styles.selected.Render("› "+head)+who)
			continue
		}
		lines = append(lines, m.styles.value.Render("  "+head)+who)
	}

	return append(lines,
		"",
		m.styles.help.Render("↑/↓ escolhe · enter entra · c cria uma sala · r atualiza · esc volta"),
	)
}

// gameView é o tabuleiro com o painel ao lado.
func (m Model) gameView() []string {
	var body string
	if m.snap == nil {
		body = m.styles.value.Render("abrindo a sala...")
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderBoard(*m.snap),
			m.styles.panel.Render(m.sidePanel(*m.snap)),
		)
	}

	return []string{
		body,
		"",
		m.move.View(),
		m.styles.help.Render("enter joga · \"ajuda\" · \"desistir\" · \"menu\" volta · ctrl+c sai"),
	}
}

// truncate encurta um texto que não cabe na coluna.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// sidePanel é a coluna ao lado do tabuleiro: quem é quem, de quem é a vez, o
// histórico e o placar de capturas.
func (m Model) sidePanel(snap room.Snapshot) string {
	var lines []string

	side := snap.YourColor.String()
	if snap.Spectator {
		side = "assistindo"
	}

	black := snap.BlackName
	if black == "" {
		black = "(vazio)"
	}

	lines = append(lines,
		m.styles.label.Render("sala:  ")+m.styles.value.Render("#"+snap.Room+" "+snap.Title),
		m.styles.label.Render("brancas:")+" "+m.styles.value.Render(snap.WhiteName),
		m.styles.label.Render("pretas: ")+" "+m.styles.value.Render(black),
		m.styles.label.Render("você:  ")+m.styles.value.Render(side),
		m.styles.label.Render("vez:   ")+m.styles.value.Render(snap.Turn.String()),
		m.styles.label.Render("status:")+" "+m.statusText(snap),
		"",
	)

	if captured := m.renderCaptured(snap.History); captured != "" {
		lines = append(lines, captured, "")
	}

	lines = append(lines,
		m.styles.label.Render("lances"),
		m.renderHistory(snap.History, historyLines),
	)

	return strings.Join(lines, "\n")
}

// statusText resume a situação da partida numa linha.
func (m Model) statusText(snap room.Snapshot) string {
	switch {
	case snap.Resigned:
		return m.styles.win.Render(fmt.Sprintf("desistência — vitória das %v", snap.Winner))

	case snap.Outcome == chess.Checkmate:
		return m.styles.win.Render(fmt.Sprintf("xeque-mate — vitória das %v", snap.Winner))

	case snap.Outcome.Over():
		return m.styles.win.Render(snap.Outcome.String())

	case snap.BlackName == "":
		return m.styles.notice.Render("esperando adversário — sala #" + snap.Room)

	case snap.Check:
		return m.styles.failure.Render("xeque!")

	case !snap.OpponentOnline:
		return m.styles.notice.Render("o adversário caiu — esperando a volta")

	case snap.Turn == snap.YourColor && !snap.Spectator:
		return m.styles.value.Render("é a sua vez")
	}

	return m.styles.label.Render("esperando o adversário")
}
