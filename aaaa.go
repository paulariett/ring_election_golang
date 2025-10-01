// Código exemplo para o trabaho de sistemas distribuidos (eleicao em anel)
// By Cesar De Rose - 2022

package main

import (
	"fmt"
	"sync"
)

type mensagem struct {
	tipo  Acao   // tipo da mensagem para fazer o controle do que fazer (eleição, confirmacao da eleicao)
	corpo [3]int // conteudo da mensagem para colocar os ids (usar um tamanho ocmpativel com o numero de processos no anel)
}

type Acao int

const (
	Failed Acao = iota + 1
	Operational
	Election
	Vote
	DefineLeader
	Erro
)

var (
	chans = []chan mensagem{ // vetor de canias para formar o anel de eleicao - chan[0], chan[1] and chan[2] ...
		make(chan mensagem),
		make(chan mensagem),
		make(chan mensagem),
		make(chan mensagem),
	}
	controle = make(chan int)
	wg       sync.WaitGroup // wg is used to wait for the program to finish
)

func MaxInt(arr [3]int) int {
	if len(arr) == 0 {
		panic("array vazio")
	}
	max := arr[0]
	for _, v := range arr {
		if v > max {
			max = v
		}
	}
	return max
}

// adiciona valor na primeira posição que esteja "vazia" (-1)
func Adicionar(arr [3]int, valor int) [3]int {
	for i := range arr {
		if arr[i] == -1 { // posição livre
			arr[i] = valor
			return arr // adicionou, sai da função
		}
	}
	return arr
}

func ElectionControler(in chan int) {
	defer wg.Done()

	var temp mensagem
	temp.corpo = [3]int{-1, -1, -1}

	// comandos para o anel iciam aqui

	// mudar o processo 0 - canal de entrada 3 - para falho (defini mensagem tipo 2 pra isto)

	temp.tipo = Failed
	chans[3] <- temp
	fmt.Printf("Controle: confirmação %d\n", <-in) // receber e imprimir confirmação

	temp.tipo = Election
	chans[0] <- temp
	fmt.Printf("Controle: confirmação %d\n", <-in) // receber e imprimir confirmação

	// matar os outrs processos com mensagens não conhecidas (só pra cosumir a leitura)
	temp.tipo = Erro
	chans[0] <- temp
	chans[1] <- temp
	chans[2] <- temp
	chans[3] <- temp

	fmt.Println("\n   Processo controlador concluído\n")
}

func ElectionStage(TaskId int, in chan mensagem, out chan mensagem, leader int) {
	defer wg.Done()

	// variaveis locais que indicam se este processo é o lider e se esta ativo

	var actualLeader int
	var bFailed bool = false            // todos inciam sem falha
	var bTriggeredElection bool = false // flag de quem iniciou a eleicao

	actualLeader = leader // indicação do lider veio por parâmatro
	for {
		temp := <-in // ler mensagem
		fmt.Printf("%2d: recebi mensagem %d, [ %d, %d, %d ]\n", TaskId, temp.tipo, temp.corpo[0], temp.corpo[1], temp.corpo[2])

		switch temp.tipo {
		case Failed: // mata o processo
			{
				bFailed = true
				fmt.Printf("%2d: estou morto!\n", TaskId)
				controle <- -5
			}
		case Operational: // revive o processo
			{
				bFailed = false
				fmt.Printf("%2d: revivi!\n", TaskId)
				controle <- -5
			}
		case Election: // dispara eleição
			{
				fmt.Printf("%2d: cheguei na eleição\n", TaskId)

				bTriggeredElection = true
				fmt.Printf("%2d: comecei eleição!\n", TaskId)

				temp.corpo = [3]int{TaskId, -1, -1}
				fmt.Printf("%2d: adicionei meu voto! [ %d, %d, %d ]\n", TaskId, temp.corpo[0], temp.corpo[1], temp.corpo[2])

				fmt.Printf("%2d: passo a mensagem para o proximo!\n", TaskId)
				// passa a mensagem com a votação
				temp.tipo = Vote
				out <- temp

				//controle <- -5
			}
		case Vote: // vota / adiciona id na mensagem
			{
				fmt.Printf("%2d: cheguei na votação\n", TaskId)

				if bTriggeredElection {
					// recebe os ids
					fmt.Printf("%2d: recebi os votos [ %d, %d, %d ]\n", TaskId, temp.corpo[0], temp.corpo[1], temp.corpo[2])

					// verifica o maior id do canal
					// joga no anel o novo lider
					actualLeader = MaxInt(temp.corpo)
					fmt.Printf("%2d: o novo lider é o processo %2d!\n", TaskId, actualLeader)

					fmt.Printf("%2d: passo a mensagem para o proximo!\n", TaskId)
					temp.corpo = [3]int{actualLeader, -1, -1}
					// passa a mensagem com temp.tipo = DefineLeader
					temp.tipo = DefineLeader
					out <- temp
				} else {
					if bFailed {
						fmt.Printf("%2d: passo a mensagem para o proximo! %2d [ %d, %d, %d ]\n", TaskId, temp.tipo, temp.corpo[0], temp.corpo[1], temp.corpo[2])
						out <- temp
					}

					if !bFailed {
						// adiciona seu id no temp.corpo
						temp.corpo = Adicionar(temp.corpo, TaskId)
						fmt.Printf("%2d: adicionei meu voto! [ %d, %d, %d ]\n", TaskId, temp.corpo[0], temp.corpo[1], temp.corpo[2])
						// passa para o proximo
						fmt.Printf("%2d: passo a mensagem para o proximo! %2d [ %d, %d, %d ]\n", TaskId, temp.tipo, temp.corpo[0], temp.corpo[1], temp.corpo[2])
						out <- temp
					}
				}

				//controle <- -5
			}
		case DefineLeader: // computar leader
			{
				fmt.Printf("%2d: cheguei pra definir o leader\n", TaskId)

				if bTriggeredElection {
					// como ja deu toda a volta, finaliza o processo de eleicao
					fmt.Printf("%2d: todos computaram o novo leader!\n", TaskId)
					fmt.Printf("%2d: eleição finalizada!\n", TaskId)
					bTriggeredElection = false

					controle <- 1
				} else {
					// pega o novo leader em temp.corpo[0]
					actualLeader = temp.corpo[0]
					fmt.Printf("%2d: recebi o novo lider %2d!\n", TaskId, actualLeader)

					// passa para o proximo
					fmt.Printf("%2d: passo a mensagem para o proximo!\n", TaskId)
					out <- temp

					//controle <- -5
				}
			}
		case Erro:
			fmt.Printf("%2d: lider atual %d\n", TaskId, actualLeader)
			return
		default:
			{
				fmt.Printf("%2d: não conheço este tipo de mensagem\n", TaskId)
				fmt.Printf("%2d: lider atual %d\n", TaskId, actualLeader)
			}
		}
		fmt.Printf("%2d: terminei \n", TaskId)
	}
}

func main() {

	wg.Add(5) // Add a count of four, one for each goroutine

	// criar os processo do anel de eleicao
	go ElectionStage(0, chans[3], chans[0], 0) // este é o lider
	go ElectionStage(1, chans[0], chans[1], 0) // não é lider, é o processo 0
	go ElectionStage(2, chans[1], chans[2], 0) // não é lider, é o processo 0
	go ElectionStage(3, chans[2], chans[3], 0) // não é lider, é o processo 0

	fmt.Println("\n   Anel de processos criado")

	// criar o processo controlador

	go ElectionControler(controle)

	fmt.Println("\n   Processo controlador criado\n")

	wg.Wait() // Wait for the goroutines to finish\
}
