// Package room tem o árbitro de uma partida.
//
// Uma sala é dona exclusiva de um *my_chess_game.RunningGame, que vive numa
// goroutine só — a goroutine árbitro. As sessões nunca tocam na partida: elas
// mandam comandos por canal e recebem de volta snapshots já copiados. É por
// isso que RunningGame.Board devolve cópia profunda.
//
// O package my_chess_game continua puro: ele não sabe o que é sessão, canal ou
// SSH. Toda a concorrência mora aqui.
package room

import (
	"errors"
	"fmt"
	"sync"
	"time"

	chess "github.com/nataferreiradev/my_chess_game"
)

// PlayerID identifica um jogador. No servidor SSH é o fingerprint da chave
// pública, que faz as vezes de conta sem exigir login.
type PlayerID string

// Listener recebe as atualizações da sala.
//
// É chamado pela goroutine árbitro, então não pode bloquear: no servidor SSH
// ele só repassa a atualização para o bubbletea com p.Send, que é rápido e
// seguro de chamar de outra goroutine.
type Listener func(Update)

// Update é uma atualização mandada às sessões: ou o estado inteiro da partida,
// ou um aviso de texto.
type Update interface{ update() }

// Snapshot é a partida inteira do ponto de vista de um jogador. Mandar o estado
// completo a cada mudança deixa a TUI burra de propósito — ela só desenha o que
// recebeu — e faz a reconexão funcionar sem nenhum caminho especial.
type Snapshot struct {
	Board     chess.Board
	Turn      chess.Color
	YourColor chess.Color
	Spectator bool

	// Room e Title identificam a sala na tela, para o jogador saber onde está
	// e poder passar o código para quem ele quer chamar.
	Room  string
	Title string
	// WhiteName e BlackName são os nomes de quem está em cada cadeira.
	// BlackName vazio quer dizer que a sala ainda espera um adversário.
	WhiteName string
	BlackName string

	Check   bool
	Outcome chess.Outcome
	// Resigned diz se a partida acabou por desistência, e Winner é a cor de
	// quem ganhou. Winner só vale quando houve mate ou desistência.
	Resigned bool
	Winner   chess.Color

	// LastMove é o lance que acabou de ser jogado, para a TUI destacar as casas.
	LastMove *chess.AppliedMove
	// History é a partida toda, na ordem. É o que um jogador que reconecta
	// recebe para reconstruir o que perdeu.
	History []chess.AppliedMove

	OpponentOnline bool
}

func (Snapshot) update() {}

// Notice é um aviso pontual para uma sessão: um erro de lance, ou um recado
// sobre o adversário.
type Notice struct {
	Text string
	// Error marca os avisos que a TUI deve destacar como erro. Um lance ilegal
	// vira um Notice de erro, nunca o fim da sessão.
	Error bool
}

func (Notice) update() {}

// abandonTimeout é quanto tempo uma sala sobrevive sem ninguém conectado antes
// de se fechar sozinha, para uma partida largada não vazar memória.
const abandonTimeout = 10 * time.Minute

// Room é uma partida com o seu árbitro. Todos os métodos podem ser chamados de
// qualquer goroutine: eles só enfileiram trabalho para o árbitro.
type Room struct {
	id    string
	title string
	cmds  chan func(*table)
	done  chan struct{}
	once  sync.Once
}

// player é um participante da sala, do ponto de vista do árbitro.
type player struct {
	id        PlayerID
	name      string
	color     chess.Color
	spectator bool
	online    bool
	listener  Listener
}

// table é o estado que só o árbitro enxerga.
type table struct {
	room     string
	title    string
	game     *chess.RunningGame
	players  map[PlayerID]*player
	seats    [2]PlayerID // indexado pela cor
	resigned bool
	winner   chess.Color
	abandon  *time.Timer
}

// ErrRoomFull diz que a sala já tem os dois jogadores sentados.
var ErrRoomFull = errors.New("a sala já tem dois jogadores")

// ErrRoomClosed diz que a sala já terminou e não aceita mais ninguém.
var ErrRoomClosed = errors.New("a sala já fechou")

// New abre uma sala com o anfitrião sentado nas brancas e a cadeira das pretas
// vazia, e já põe o árbitro de pé. O adversário chega depois, por Sit.
//
// A sala nasce pela metade de propósito: quem cria anuncia a partida no lobby e
// espera; enquanto ninguém senta, o tabuleiro já existe e já pode ser desenhado.
func New(id, title string, host PlayerID, hostName string) *Room {
	r := &Room{
		id:    id,
		title: title,
		cmds:  make(chan func(*table), 32),
		done:  make(chan struct{}),
	}

	t := &table{
		room:  id,
		title: title,
		game:  chess.NewGame(),
		players: map[PlayerID]*player{
			host: {id: host, name: hostName, color: chess.White},
		},
		seats: [2]PlayerID{chess.White: host},
	}

	go r.run(t)
	return r
}

// ID devolve o identificador da sala.
func (r *Room) ID() string { return r.id }

// Title devolve o nome que a sala mostra no lobby.
func (r *Room) Title() string { return r.title }

// Sit senta o adversário na cadeira das pretas.
//
// Devolve ErrRoomFull se a cadeira já estiver ocupada — nesse caso quem chegou
// pode entrar como espectador, por Join. Sentar de novo quem já está na sala
// não é erro: é o caminho de quem reconecta.
func (r *Room) Sit(id PlayerID, name string) error {
	result := make(chan error, 1)

	if !r.do(func(t *table) {
		switch {
		case t.players[id] != nil && !t.players[id].spectator:
			result <- nil // já tem cadeira nesta sala
		case t.seats[chess.Black] != "":
			result <- ErrRoomFull
		default:
			t.players[id] = &player{id: id, name: name, color: chess.Black}
			t.seats[chess.Black] = id
			result <- nil
		}
	}) {
		return ErrRoomClosed
	}

	// A sala pode fechar entre o comando entrar na fila e o árbitro chegar
	// nele: sem esperar também pelo done, quem chamou ficaria preso aqui.
	select {
	case err := <-result:
		return err
	case <-r.done:
		return ErrRoomClosed
	}
}

// Closed fecha quando a sala termina. O lobby usa isso para recolher a sala.
func (r *Room) Closed() <-chan struct{} { return r.done }

// Close encerra a sala. Pode ser chamado mais de uma vez.
func (r *Room) Close() {
	r.once.Do(func() { close(r.done) })
}

// run é a goroutine árbitro: a única dona do RunningGame.
func (r *Room) run(t *table) {
	defer func() {
		// Um erro num comando não pode derrubar o servidor junto com a sala.
		if rec := recover(); rec != nil {
			t.broadcastNotice(fmt.Sprintf("a sala fechou por um erro interno: %v", rec), true)
		}
		if t.abandon != nil {
			t.abandon.Stop()
		}
		r.Close()
	}()

	for {
		select {
		case fn := <-r.cmds:
			fn(t)
		case <-r.done:
			return
		}
	}
}

// do enfileira um comando para o árbitro. Devolve false se a sala já fechou.
func (r *Room) do(fn func(*table)) bool {
	select {
	case r.cmds <- fn:
		return true
	case <-r.done:
		return false
	}
}

// Join conecta (ou reconecta) um participante e manda o estado atual da
// partida. Quem não tem cadeira na sala entra como espectador.
func (r *Room) Join(id PlayerID, name string, listener Listener) {
	r.do(func(t *table) {
		p, ok := t.players[id]
		if !ok {
			p = &player{id: id, name: name, spectator: true}
			t.players[id] = p
		}

		reconnect := p.online
		p.listener = listener
		p.online = true
		t.cancelAbandon()

		if !p.spectator && !reconnect {
			t.noticeOthers(id, fmt.Sprintf("%s entrou na partida.", p.name), false)
		}

		// Todo mundo recebe a partida inteira, histórico incluído: é assim que
		// quem reconecta reconstrói o que perdeu, e é como os outros ficam
		// sabendo que ele voltou.
		t.broadcast(nil)
	})
}

// Leave marca o participante como desconectado, sem encerrar a partida: ele
// ainda pode voltar e retomar de onde parou.
func (r *Room) Leave(id PlayerID) {
	r.do(func(t *table) {
		p, ok := t.players[id]
		if !ok || !p.online {
			return
		}

		p.online = false
		p.listener = nil

		if p.spectator {
			delete(t.players, id)
		} else {
			t.noticeOthers(id, fmt.Sprintf("%s caiu. A partida espera a volta dele.", p.name), false)
		}

		t.broadcast(nil)
		t.scheduleAbandonCheck(r)
	})
}

// Move manda um lance em notação algébrica. Um lance recusado vira um aviso de
// erro para quem jogou, e mais nada: a partida e a sessão seguem de pé.
func (r *Room) Move(id PlayerID, notation string) {
	r.do(func(t *table) {
		p, ok := t.players[id]
		if !ok {
			return
		}

		switch {
		case p.spectator:
			t.send(p, Notice{Text: "você está assistindo, não jogando.", Error: true})
			return
		case t.seats[chess.Black] == "":
			t.send(p, Notice{Text: "a sala ainda espera um adversário.", Error: true})
			return
		case t.over():
			t.send(p, Notice{Text: "a partida já acabou.", Error: true})
			return
		case t.game.CurrentPlayer() != p.color:
			t.send(p, Notice{Text: "não é a sua vez.", Error: true})
			return
		}

		applied, err := t.game.MakeMove(notation)
		if err != nil {
			t.send(p, Notice{Text: err.Error(), Error: true})
			return
		}

		if t.game.Outcome() == chess.Checkmate {
			t.winner = p.color
		}

		t.broadcast(&applied)
	})
}

// Resign faz o jogador desistir, encerrando a partida a favor do adversário.
func (r *Room) Resign(id PlayerID) {
	r.do(func(t *table) {
		p, ok := t.players[id]
		if !ok || p.spectator || t.over() {
			return
		}

		t.resigned = true
		t.winner = opponent(p.color)

		t.broadcastNotice(fmt.Sprintf("%s desistiu. Vitória das %v.", p.name, t.winner), false)
		t.broadcast(nil)
	})
}

// --- estado interno, só o árbitro mexe --------------------------------------

func (t *table) over() bool {
	return t.resigned || t.game.Outcome().Over()
}

// send entrega uma atualização a um participante conectado.
func (t *table) send(p *player, update Update) {
	if p == nil || !p.online || p.listener == nil {
		return
	}
	p.listener(update)
}

// broadcast manda a todos o estado da partida, cada um do seu ponto de vista.
func (t *table) broadcast(last *chess.AppliedMove) {
	for _, p := range t.players {
		t.send(p, t.snapshotFor(p, last))
	}
}

// broadcastNotice manda um aviso a todos.
func (t *table) broadcastNotice(text string, isError bool) {
	for _, p := range t.players {
		t.send(p, Notice{Text: text, Error: isError})
	}
}

// noticeOthers manda um aviso a todos menos a um.
func (t *table) noticeOthers(except PlayerID, text string, isError bool) {
	for id, p := range t.players {
		if id == except {
			continue
		}
		t.send(p, Notice{Text: text, Error: isError})
	}
}

// snapshotFor monta o estado da partida do ponto de vista de um participante.
// O tabuleiro e o histórico já saem copiados do RunningGame, então cada sessão
// recebe dados só seus.
func (t *table) snapshotFor(p *player, last *chess.AppliedMove) Snapshot {
	return Snapshot{
		Board:          t.game.Board(),
		Turn:           t.game.CurrentPlayer(),
		YourColor:      p.color,
		Spectator:      p.spectator,
		Room:           t.room,
		Title:          t.title,
		WhiteName:      t.seatName(chess.White),
		BlackName:      t.seatName(chess.Black),
		Check:          t.game.InCheck(),
		Outcome:        t.game.Outcome(),
		Resigned:       t.resigned,
		Winner:         t.winner,
		LastMove:       last,
		History:        t.game.History(),
		OpponentOnline: t.opponentOnline(p),
	}
}

// opponentOnline diz se o adversário está conectado. Para um espectador, diz se
// os dois jogadores estão.
func (t *table) opponentOnline(p *player) bool {
	if p.spectator {
		return t.seatOnline(chess.White) && t.seatOnline(chess.Black)
	}
	return t.seatOnline(opponent(p.color))
}

// seatName devolve o nome de quem está numa cadeira, ou "" se ela está vazia.
func (t *table) seatName(c chess.Color) string {
	p, ok := t.players[t.seats[c]]
	if !ok {
		return ""
	}
	return p.name
}

func (t *table) seatOnline(c chess.Color) bool {
	other, ok := t.players[t.seats[c]]
	return ok && other.online
}

// scheduleAbandonCheck marca a sala para fechar se ninguém voltar. Uma partida
// largada não pode segurar a goroutine árbitro para sempre.
func (t *table) scheduleAbandonCheck(r *Room) {
	if t.anyoneOnline() {
		return
	}

	// Uma sala que nunca chegou a ter adversário e perdeu o anfitrião não tem
	// por que continuar anunciada no lobby.
	if t.seats[chess.Black] == "" {
		r.Close()
		return
	}

	// Acabada e vazia, a sala não serve mais para nada.
	if t.over() {
		r.Close()
		return
	}

	t.cancelAbandon()
	t.abandon = time.AfterFunc(abandonTimeout, func() {
		r.do(func(t *table) {
			if !t.anyoneOnline() {
				r.Close()
			}
		})
	})
}

func (t *table) cancelAbandon() {
	if t.abandon != nil {
		t.abandon.Stop()
		t.abandon = nil
	}
}

func (t *table) anyoneOnline() bool {
	for _, p := range t.players {
		if p.online {
			return true
		}
	}
	return false
}

func opponent(c chess.Color) chess.Color {
	if c == chess.White {
		return chess.Black
	}
	return chess.White
}
