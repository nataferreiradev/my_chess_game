// Package lobby é a sala de entrada do servidor.
//
// Quem chega não é mais pareado às cegas: cria uma sala, que fica anunciada
// para os outros, ou escolhe uma das salas abertas na lista. Quem já tem uma
// partida em andamento volta para ela — é o caminho da reconexão.
package lobby

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nataferreiradev/my_chess_game/internal/room"
)

// ErrNotFound diz que não existe sala com aquele código: ela pode ter fechado
// entre a lista que o jogador viu e a hora em que ele escolheu.
var ErrNotFound = errors.New("essa sala não existe mais")

// ErrWrongPassword diz que a sala é trancada e a senha não confere. É o mesmo
// erro para quem erra a senha e para quem não manda nenhuma: de fora não dá
// para distinguir os dois casos.
var ErrWrongPassword = errors.New("senha errada")

// Lobby guarda as salas abertas e quem está em qual.
//
// Aqui um mutex basta: as operações são curtas e não tocam em nenhuma partida.
// Quem é dona da partida é a goroutine árbitro da sala.
type Lobby struct {
	mu    sync.Mutex
	open  map[string]*entry
	rooms map[room.PlayerID]*room.Room
	seq   int
}

// entry é uma sala anunciada, do ponto de vista do lobby. O lobby guarda os
// nomes em vez de perguntar à sala para montar a lista sem falar com nenhum
// árbitro — listar não pode depender de nenhuma partida estar respondendo.
type entry struct {
	room    *room.Room
	code    string
	title   string
	host    string
	guest   string
	created time.Time

	// pass é o hash da senha da sala, e locked diz se ela tem senha. Guardamos
	// o hash em vez do texto para a senha não aparecer num dump de memória nem
	// num log — ela não protege dinheiro, mas é a senha que alguém reusou.
	pass   [32]byte
	locked bool
}

// Open é uma sala como ela aparece na lista.
type Open struct {
	Code  string
	Title string
	Host  string
	Guest string
	// Locked diz que a sala pede senha para entrar.
	Locked bool
	// Waiting diz que a cadeira das pretas está vazia: quem entrar joga.
	// Numa sala cheia só dá para assistir.
	Waiting bool
	Since   time.Time
}

// New devolve um lobby vazio.
func New() *Lobby {
	return &Lobby{
		open:  make(map[string]*entry),
		rooms: make(map[room.PlayerID]*room.Room),
	}
}

// Create abre uma sala nova com o jogador de anfitrião, jogando de brancas, e a
// anuncia na lista. title vazio vira um nome a partir do nome do jogador.
//
// password vazia deixa a sala aberta a qualquer um; com senha, a sala continua
// aparecendo na lista — marcada como trancada — mas só entra quem souber a
// senha, jogador ou espectador.
//
// Quem já está numa partida recebe a partida de volta em vez de uma sala nova:
// nenhum jogador pode estar em duas ao mesmo tempo.
func (l *Lobby) Create(id room.PlayerID, name, title, password string) *room.Room {
	l.mu.Lock()
	defer l.mu.Unlock()

	if r, ok := l.rooms[id]; ok {
		return r
	}

	l.seq++
	code := fmt.Sprintf("%d", l.seq)
	if title == "" {
		title = "sala de " + name
	}

	r := room.New(code, title, id, name)
	e := &entry{room: r, code: code, title: title, host: name, created: time.Now()}
	if password = strings.TrimSpace(password); password != "" {
		e.pass, e.locked = sha256.Sum256([]byte(password)), true
	}
	l.open[code] = e
	l.rooms[id] = r

	go l.reap(r, code)

	return r
}

// Join põe o jogador numa sala já anunciada: sentado nas pretas se a cadeira
// estiver vazia, de espectador se a partida já tiver dois jogadores.
//
// Quem volta para a própria partida em andamento entra de novo nela, sem erro.
func (l *Lobby) Join(code string, id room.PlayerID, name, password string) (*room.Room, error) {
	l.mu.Lock()
	e, ok := l.open[code]
	mine := l.rooms[id]
	l.mu.Unlock()

	if !ok {
		return nil, ErrNotFound
	}
	if mine == e.room {
		return e.room, nil // reconexão à própria partida: já passou pela senha
	}
	if mine != nil {
		return nil, errors.New("você já está numa partida")
	}
	if !e.allows(password) {
		return nil, ErrWrongPassword
	}

	switch err := e.room.Sit(id, name); {
	case err == nil:
		l.mu.Lock()
		e.guest = name
		l.rooms[id] = e.room
		l.mu.Unlock()

	case errors.Is(err, room.ErrRoomFull):
		// Sem cadeira sobrando, quem chegou assiste. Espectador não fica
		// registrado em rooms: ele não tem partida para reconectar.

	case errors.Is(err, room.ErrRoomClosed):
		return nil, ErrNotFound

	default:
		return nil, err
	}

	return e.room, nil
}

// allows diz se a senha dada abre a sala. A comparação é de tempo constante
// por hábito, não por necessidade: é o jeito certo de comparar segredo.
func (e *entry) allows(password string) bool {
	if !e.locked {
		return true
	}

	given := sha256.Sum256([]byte(strings.TrimSpace(password)))
	return subtle.ConstantTimeCompare(given[:], e.pass[:]) == 1
}

// Current devolve a partida em que o jogador está, se houver. É o que faz uma
// sessão que reconecta cair direto no tabuleiro em vez de na tela inicial.
func (l *Lobby) Current(id room.PlayerID) *room.Room {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rooms[id]
}

// List devolve as salas abertas, as que esperam adversário primeiro e, dentro
// de cada grupo, as mais antigas antes.
func (l *Lobby) List() []Open {
	l.mu.Lock()
	defer l.mu.Unlock()

	list := make([]Open, 0, len(l.open))
	for _, e := range l.open {
		list = append(list, Open{
			Code:    e.code,
			Title:   e.title,
			Host:    e.host,
			Guest:   e.guest,
			Locked:  e.locked,
			Waiting: e.guest == "",
			Since:   e.created,
		})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Waiting != list[j].Waiting {
			return list[i].Waiting
		}
		return list[i].Since.Before(list[j].Since)
	})

	return list
}

// Leave avisa a sala que o jogador caiu.
//
// A sala dele continua registrada de propósito: é o que faz a reconexão levar
// de volta para a mesma partida. Quem tira o registro é reap, quando a sala
// fecha de verdade.
func (l *Lobby) Leave(id room.PlayerID) {
	l.mu.Lock()
	r, ok := l.rooms[id]
	l.mu.Unlock()

	if ok {
		r.Leave(id)
	}
}

// reap tira a sala da lista e solta os jogadores quando ela fecha, para eles
// poderem entrar numa partida nova.
func (l *Lobby) reap(r *room.Room, code string) {
	<-r.Closed()

	l.mu.Lock()
	defer l.mu.Unlock()

	if e, ok := l.open[code]; ok && e.room == r {
		delete(l.open, code)
	}
	for id, other := range l.rooms {
		if other == r {
			delete(l.rooms, id)
		}
	}
}

// Rooms devolve quantas salas estão abertas.
func (l *Lobby) Rooms() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.open)
}
