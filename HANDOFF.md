# my_chess_game — o que falta para virar o servidor

Documento de passagem. Estado verificado em 2026-09-12 lendo **e executando** o código.
`my_chess_game` será o servidor: processa lances, transmite aos clientes, gerencia sessões e salas.
O cliente é o `ssh` puro do usuário — ele não roda código nosso.

---

## 0. Bloqueador de ambiente

O toolchain Go da máquina está inconsistente: `/usr/lib/golang/VERSION` diz `go1.25.11` mas
pertence ao RPM 1.26.7, e sobraram arquivos de transação interrompida (`go;6a9627a5`) em
`/usr/lib/golang/bin/`. O binário é 1.26.7 mas se identifica como 1.25.11 e compila o src
com as regras erradas: `asm: ABI selector only permitted when compiling runtime`.

```sh
sudo dnf reinstall golang golang-bin golang-src
```

Enquanto não for feito, dá para compilar com o toolchain íntegro já baixado:

```sh
export GOROOT=/home/nata/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64
$GOROOT/bin/go test ./...
```

---

## 1. Estado atual da API (pós-Fase 1)

Módulo `github.com/nataferreiradev/my_chess_game`, package `my_chess_game`.

```go
const BoardSize = 64

type Color byte              // Black=0, White=1; Color.String() → "brancas"/"pretas"
type GamePiece struct { Piece parser.Piece; Color Color }
type Board []*GamePiece      // 64 casas, nil = vazia. Índice 0 = a8, índice 63 = h1.

func NewBoard() Board

type RunningGame struct{ /* playingColor, board — privados */ }

func NewGame() *RunningGame
func (g *RunningGame) Board() Board                    // CÓPIA PROFUNDA
func (g *RunningGame) PieceAt(idx int) (GamePiece, bool) // false = vazia ou fora de faixa
func (g *RunningGame) CurrentPlayer() Color
func (g *RunningGame) MakeMove(cmd string) error
```

`Board()` devolve cópia profunda **de propósito**: é o snapshot que a goroutine árbitro manda
para as goroutines das sessões sem compartilhar `*RunningGame`. Não troque por cópia rasa —
`Board` é `[]*GamePiece` e a cópia rasa deixaria a TUI mutar as peças da partida.

`running_game_test.go` tem 8 testes cobrindo isso. Eles passam.

---

## 2. O que `MakeMove` faz hoje: quase nada

Ela chama `parser.Parse`, descarta o resultado, alterna a cor e devolve nil.
**Nunca escreve no tabuleiro.** Todo o motor de xadrez está por fazer.

O `TODO(fase-futura)` dentro dela marca o ponto de entrada.

---

## 3. Motor de xadrez — ordem sugerida

### Etapa A — fundação (nada disso existe)

**A1. Conversão `parser.Square` ↔ índice.** Faça isto primeiro, com teste. É a fonte de bug
mais provável do projeto, porque as duas convenções são invertidas:
`Square{File, Rank}` tem Rank 0 = linha 1, mas o índice 0 = a8 (linha 8).

```
idx = (7 - sq.Rank)*8 + sq.File
```
Confira: a8 → 0, e4 → 36, h1 → 63.

**A2. Alcance por peça** — algo como `canReach(b Board, from, to int) bool`.
O peão é o caso chato e a direção **contraintuitiva** por causa da indexação:
brancas andam para índices **menores** (`-8`), pretas para **maiores** (`+8`).
Avanço duplo só da fileira inicial (brancas 48..55, pretas 8..15); captura só na diagonal
e só havendo peça inimiga (ou en passant).

**A3. Bloqueio de caminho** para bispo/torre/dama.

**A4. Resolução da casa de origem.** O `Parse` devolve `To` completo mas `From` quase sempre
`{-1,-1}`. É preciso varrer as 64 casas procurando peças da cor da vez, do tipo certo, que
alcancem o destino. Zero candidatos → erro "nenhuma peça alcança"; mais de um → erro
"lance ambíguo". **Cuidado:** a desambiguação do parser está quebrada (ver Etapa D).

**A5. Validação de turno.** Hoje `playingColor` alterna mas nunca é conferido contra a cor da
peça movida. Dá para mover peça do adversário.

**A6. Captura.** Aplicar, e validar coerência com o flag `Move.Capture`.

### Etapa B — estado novo em `RunningGame`

- **B1.** Direitos de roque: flags de rei e de cada torre já movidos.
- **B2.** Casa de en passant disponível (válida só no lance imediatamente seguinte).
- **B3.** Contador da regra dos 50 lances + histórico de posições para repetição tripla.
- **B4.** Histórico de lances. Precisa existir de qualquer jeito para o servidor: é o que
  permite reenviar a partida para quem reconecta ou entra como espectador.

### Etapa C — xeque e fim de jogo

- **C1.** `isSquareAttacked(b Board, idx int, by Color) bool` — base de tudo que vem abaixo.
- **C2.** Legalidade: gerar o lance pseudo-legal, aplicar numa cópia, e rejeitar se o próprio
  rei ficar atacado.
- **C3.** Mate e afogamento: sem lance legal e rei atacado = mate; sem lance legal e rei não
  atacado = afogamento.
- **C4.** Empates: 50 lances, repetição tripla, material insuficiente.

### Etapa D — `chess_notation_parser` (repo separado, bloqueia B1/roque e promoção)

Bugs confirmados executando o parser:

| Entrada | Resultado atual | Problema |
|---|---|---|
| `Nbd2` | `Nd2`, `From={-1,-1}` | desambiguação **descartada em silêncio**, sem erro |
| `R1a3` | `Ra3`, `From={-1,-1}` | idem |
| `Ne4e5` | `Ne5`, sem erro | lixo antes do destino é ignorado |
| `O-O`, `O-O-O` | erro | roque não existe |
| `e8=Q` | erro | promoção não existe |
| `e4!?` | erro | anotações não são toleradas |

`From.Rank` **nunca** é preenchido (`parser.go:67-72` só trata `File`). Conserte isto antes
de confiar em qualquer desambiguação, senão A4 vai rejeitar lances legais como ambíguos.

---

## 4. Sessões e salas — recomendações de arquitetura

**Mantenha as regras de xadrez puras.** O package `my_chess_game` não deve importar `net`,
`bubbletea`, nem ter canais. Sessões, salas e lobby vão em packages separados do mesmo módulo
(ex.: `internal/lobby`, `internal/room`). `RunningGame` não pode saber o que é sessão SSH.

- **Árbitro:** uma goroutine dona exclusiva de `*RunningGame`. Sessões falam por canais e
  recebem snapshots via `p.Send(...)`. `Board()` já foi feito para isso.
- **Identidade:** fingerprint da chave pública (`session.PublicKey()`), sem login.
- **Salas:** mapa sala → 2 jogadores + espectadores, com mutex ou uma goroutine própria de lobby.
- **Reconexão:** com o histórico de lances (B4), reconectar é só reenviar o histórico.
- **Uma sessão que quebra não pode derrubar o servidor:** `recover` por sessão, erros retornados.

---

## 5. Duas decisões que vale tomar cedo

**5.1. `MakeMove` deveria devolver o que mudou.** Hoje é `(error)`. Para transmitir aos dois
clientes, o árbitro precisa saber *o quê* mudou, não só que deu certo. Considere:

```go
type AppliedMove struct {
    From, To int
    Piece    GamePiece
    Captured *GamePiece // nil se não houve captura
    Castle   bool
    Promotion parser.Piece
    Check, Checkmate bool
}

func (g *RunningGame) MakeMove(cmd string) (AppliedMove, error)
```

Serve ao histórico (B4) e à transmissão. Mudar isso depois custa caro.

**5.2. FEN import/export.** Não existe. Sem ele, todo teste de posição precisa jogar 30 lances
até chegar lá. Com ele, testar roque, en passant e mate vira uma linha. Recomendo forte, cedo.

**5.3. Menor, mas útil:** símbolo Unicode da peça (`♜♞♝♛♚♟`) para a TUI renderizar. Pode ficar
na lib ou no servidor, mas alguém vai precisar.

---

## 6. Restrições do projeto (não negociáveis)

- Go idiomático, sem framework além do Charm.
- **Não** trocar a lógica de xadrez por `notnil/chess` ou outra lib de terceiros — o ponto do
  projeto é usar as duas libs próprias.
- Erros retornados, nunca panic.
- Testes para toda lógica nova de jogo. A camada de TUI pode ficar sem teste.
- Comentários e mensagens ao usuário em português.
