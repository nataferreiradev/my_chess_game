// Command xadrez-ssh serve partidas de xadrez por SSH.
//
// O cliente é o ssh do próprio jogador: ele não instala nem roda código nosso.
// Basta `ssh -p 23234 localhost` para cair na tela inicial, onde o jogador
// escolhe o nome e cria uma sala ou entra numa partida já aberta.
//
//	xadrez-ssh [-addr host:porta] [-hostkey caminho]
package main

import (
	"context"
	"errors"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wish/recover"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"

	"github.com/nataferreiradev/my_chess_game/internal/lobby"
	"github.com/nataferreiradev/my_chess_game/internal/room"
	"github.com/nataferreiradev/my_chess_game/internal/tui"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:23234", "endereço onde o servidor escuta")
	hostKey := flag.String("hostkey", ".ssh/xadrez_ed25519", "caminho da chave de host (criada se não existir)")
	flag.Parse()

	if err := run(*addr, *hostKey); err != nil {
		log.Error("o servidor parou", "erro", err)
		os.Exit(1)
	}
}

func run(addr, hostKey string) error {
	server, err := newServer(addr, hostKey, lobby.New())
	if err != nil {
		return err
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("xadrez por ssh no ar", "endereço", addr)
		log.Info("para jogar", "comando", "ssh -p "+port(addr)+" localhost")

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) && !errors.Is(err, http.ErrServerClosed) {
			log.Error("o servidor caiu", "erro", err)
			done <- syscall.SIGTERM
		}
	}()

	<-done

	log.Info("encerrando, esperando as sessões abertas")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return err
	}
	return nil
}

// newServer monta o servidor SSH com todas as camadas no lugar.
func newServer(addr, hostKey string, hall *lobby.Lobby) (*ssh.Server, error) {
	return wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(hostKey),

		// Sem login: qualquer chave entra, e é ela que identifica o jogador.
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithKeyboardInteractiveAuth(func(ssh.Context, gossh.KeyboardInteractiveChallenge) bool { return true }),

		wish.WithIdleTimeout(30*time.Minute),

		wish.WithMiddleware(
			// A ordem importa: o wish aplica de trás para frente, então o
			// recover embrulha tudo e uma sessão que entra em pânico não leva
			// o servidor junto.
			//
			// O perfil tem de ser colorido: as peças dos dois lados usam a
			// mesma figura cheia e só se distinguem pela cor do texto. Com
			// termenv.Ascii o servidor apaga toda a cor na saída e as brancas
			// ficam idênticas às pretas.
			bm.MiddlewareWithProgramHandler(handler(hall), termenv.ANSI256),
			activeterm.Middleware(), // sem PTY não há TUI
			logging.Middleware(),
			recover.Middleware(),
		),
	)
}

// handler monta a TUI de cada sessão.
func handler(hall *lobby.Lobby) bm.ProgramHandler {
	return func(sess ssh.Session) *tea.Program {
		id, name := identify(sess)

		// O renderer sai da sessão, então o perfil de cor é o do terminal do
		// cliente, e não o do servidor.
		model, seat := tui.New(sess.Context(), hall, id, name, bm.MakeRenderer(sess))

		program := tea.NewProgram(model, bm.MakeOptions(sess)...)
		seat.Attach(program)

		log.Info("sessão nova", "jogador", name, "de", sess.RemoteAddr())
		return program
	}
}

// identify devolve quem é o jogador.
//
// A conta é o fingerprint da chave pública: sem cadastro, sem senha, e a mesma
// chave sempre volta para a mesma partida. Quem entrar sem chave joga como
// convidado, com uma identidade que vale só para aquela sessão.
func identify(sess ssh.Session) (room.PlayerID, string) {
	name := sess.User()
	if name == "" {
		name = "convidado"
	}

	if key := sess.PublicKey(); key != nil {
		return room.PlayerID(gossh.FingerprintSHA256(key)), name
	}

	return room.PlayerID("convidado:" + sess.RemoteAddr().String() + ":" + sess.Context().SessionID()), name
}

// port extrai a porta de um endereço, para a mensagem de boas-vindas do log.
func port(addr string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil {
		return p
	}
	return addr
}
