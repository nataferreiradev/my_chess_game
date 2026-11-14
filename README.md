# my_chess_game

Servidor de xadrez jogável por SSH, no estilo do terminal.shop: o jogador roda
`ssh` e cai direto numa TUI, sem instalar nada.

O cliente é o `ssh` do próprio jogador — ele não roda código nosso. Regras,
TUI e salas rodam todas aqui. A notação que o jogador digita trafega como texto
até o servidor, que parseia e valida.

## Rodar

```sh
go run ./cmd/xadrez-ssh
```

E, de outro terminal (dois, para jogar contra você mesmo):

```sh
ssh -p 23234 localhost
```

Quem conecta cai na tela inicial: escolhe o nome e então **cria uma sala** ou
**busca partidas** — a lista mostra as salas esperando adversário e as que já
estão em jogo, que dá para assistir. Quem cria a sala joga com as brancas.

Ao criar, dá para dar um nome à sala e **uma senha**. A sala com senha continua
aparecendo na lista, marcada com 🔒, mas quem escolher ela — jogador ou
espectador — tem de digitar a senha para entrar. Senha vazia deixa a sala
aberta a qualquer um.

| flag | padrão | o que faz |
|---|---|---|
| `-addr` | `0.0.0.0:23234` | endereço onde o servidor escuta |
| `-hostkey` | `.ssh/xadrez_ed25519` | chave de host, criada se não existir |

Nos menus: `↑`/`↓` escolhem, `enter` confirma, e o número da linha faz o mesmo
de uma vez. Na partida: digite o lance em notação algébrica (`e4`, `Nf3`,
`exd5`, `O-O`, `e8=Q`, `Bxe4+`) e aperte enter. `ajuda` explica, `desistir`
entrega a partida, `menu` volta para a tela inicial, `figuras` troca o desenho
das peças, `ctrl+c` sai. Lance recusado vira mensagem na tela — nunca derruba a
sessão.

Cada lado tem o seu desenho — as brancas vazadas (`♔♕♖`), as pretas cheias
(`♚♛♜`) — e ainda por cima a cor do texto separa os dois, por isso o servidor
força no mínimo 256 cores na sessão. Numa fonte sem as figuras vazadas, que as
desenha com largura errada e desalinha o tabuleiro, `figuras` passa os dois
lados para a figura cheia, separados só pela cor.

## Como está organizado

O package raiz (`my_chess_game`) é **puro**: só as regras do xadrez. Não importa
`net`, não tem canal, não sabe o que é sessão SSH. Toda a concorrência mora nos
packages de dentro.

| package | papel |
|---|---|
| `my_chess_game` | motor de xadrez: tabuleiro, lances, legalidade, fim de jogo, FEN |
| `internal/room` | o árbitro: uma goroutine dona exclusiva de um `*RunningGame` |
| `internal/lobby` | as salas anunciadas: criar, listar, entrar, reconectar |
| `internal/engine` | o jogador de computador: escolhe o lance. **Só o esqueleto** — ver `PROMPT_ENGINE.md` |
| `internal/tui` | a TUI de uma sessão, em bubbletea |
| `cmd/xadrez-ssh` | o servidor SSH, em wish |

A sessão nunca toca na partida: ela manda comandos para a sala por canal e
recebe de volta snapshots já copiados. É por isso que `Board()` devolve cópia
profunda — não troque por cópia rasa.

## O motor

```go
game := my_chess_game.NewGame()
applied, err := game.MakeMove("e4")   // AppliedMove descreve tudo que mudou
```

Cobre lance e captura de todas as peças, bloqueio de caminho, roque (com
direitos, caminho livre e as três casas de xeque), en passant, promoção nas
quatro peças, xeque, cravada, mate, afogamento, e os empates por 50 lances,
repetição tripla e material insuficiente.

`NewGameFromFEN` e `FEN()` fazem import e export de posição, o que permite
testar roque ou mate numa linha em vez de jogar 30 lances até a posição.

A correção do gerador de lances é verificada por **perft** em quatro posições
conhecidas — inclusive a Kiwipete e a posição de promoções —, que é o teste
padrão da área: os números só batem se roque, en passant, promoção, cravada e
xeque estiverem todos certos.

### Coordenadas

As duas convenções do projeto são invertidas, e essa é a armadilha do código:

- `parser.Square{File, Rank}` tem `Rank 0` = linha 1;
- o índice do tabuleiro tem `0` = a8 (linha 8) e `63` = h1.

A conversão é `idx = (7-Rank)*8 + File` — a8 → 0, e4 → 36, h1 → 63 — e está em
`SquareIndex`/`IndexSquare`. Consequência: **peão branco anda para índices
menores** (`-8`) e o preto para maiores (`+8`).

## A lib de notação

`github.com/nataferreiradev/chess_notation_parser` é usada só pelo servidor.
Hoje o `go.mod` aponta para a cópia local:

```
replace github.com/nataferreiradev/chess_notation_parser => ../chess_notation_parser
```

Isso existe porque o parser foi corrigido junto com este servidor (roque,
promoção, `From.Rank`, desambiguação de peça maior). Quando essas mudanças forem
publicadas, troque o `replace` pela versão nova.

## Testes

```sh
go test ./...
go test -race ./...
```

`cmd/xadrez-ssh/e2e_test.go` é o critério de aceite automatizado: sobe o
servidor de verdade, conecta clientes SSH de verdade com PTY, passa pela tela
inicial, cria e acha a sala, joga uma sequência legal, leva um lance ilegal recusado sem cair, e chega a um
mate detectado nos dois terminais.
