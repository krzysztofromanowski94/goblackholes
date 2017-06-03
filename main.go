package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/krzysztofromanowski94/agentproto"
	"net"
	"math"
	"runtime"
	"sync"
	"time"
	"os"
	"log"
)

var (
	agentAmount         int                        = 500
	singleServiceAmount int                        = 50
	slowmotion          int                        = 0 // every step sleeps for this amount of milliseconds
	border              Border                     = Border{-1.5, -3.0, 4.0, 4.0, 0, 0, 0, 0}
	agentList           []*Agent                   = make([]*Agent, agentAmount, agentAmount)
	bestAgent           BestAgent                  = BestAgent{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64, 0, 0, sync.Mutex{}}
	newAgentChannel     chan *Agent                = make(chan *Agent, agentAmount)
	fitnessChannel      chan *Agent                = make(chan *Agent, agentAmount)
	getBestChannel      chan *Agent                = make(chan *Agent, agentAmount)
	moveChannel         chan *Agent                = make(chan *Agent, agentAmount)
	eventHorizonChannel chan *Agent                = make(chan *Agent, agentAmount)
	protoChannel        chan *Agent                = make(chan *Agent, agentAmount)
	randomBuffer        chan float64               = make(chan float64, agentAmount*5)
	maxAccuracy         chan bool                  = make(chan bool, 1)
	endComputing        chan bool                  = make(chan bool, singleServiceAmount+10)
	exitProgram         chan bool                  = make(chan bool, 1)
	typeOfFunction      TypeOfFunction_S           = TypeOfFunction_S{Rastrigin: true}
	conn                net.Conn
	//typeOfFunction TypeOfFunction_S = TypeOfFunction_S{Rosenbrock: true}
	//typeOfFunction TypeOfFunction_S = TypeOfFunction_S{Easom: true}
	//typeOfFunction TypeOfFunction_S = TypeOfFunction_S{McCormick: true}
	//typeOfFunction TypeOfFunction_S = TypeOfFunction_S{StringEvaluation: "20 + pow(x, 2) + pow(y, 2) - 10 * (cos(2 * PI() * x) + cos(2 * PI() * y))"}
	i int
)

func main() {

	StartComputing()

	Utils()

	time.Sleep(10 * time.Second)
	return

	for {
		select {
		case <-exitProgram:
			return
		}
	}
}

func StartComputing() {
	for i := 0; i < singleServiceAmount; i++ {
		go func() {
			for {
				select {
				case <-endComputing:
					return
				default:
					getBest(getBestChannel, <-fitnessChannel)
				}
			}
		}()
		go func() {
			for {
				select {
				case <-endComputing:
					return
				default:
					move(moveChannel, <-getBestChannel)
				}
			}
		}()
		go func() {
			for {
				select {
				case <-endComputing:
					return
				default:
					eventHorizon(eventHorizonChannel, <-moveChannel)
				}
			}
		}()
		go func() {
			for {
				select {
				case <-endComputing:
					return
				default:
					//countFitness(fitnessChannel, <-eventHorizonChannel)
					countFitness(protoChannel, <-eventHorizonChannel)
				}
			}
		}()
		go func() {
			for {
				select {
				case <-endComputing:
					return
				default:
					tA := <-protoChannel
					fitnessChannel <- tA
					toSend := new (agentproto.AgentData)
					toSend.X = proto.Float64(tA.x)
					toSend.Y = proto.Float64(tA.y)
					toSend.Fitness = proto.Float64(tA.fitness)
					toSend.Average = proto.Int32(int32(tA.times))
					if stuff, err := proto.Marshal(toSend) ; err != nil {
						log.Println(err)
						os.Exit(-1)
					} else {
						conn.Write(stuff)
					}
					//toSend.Best = proto.Bool
					//countFitness(fitnessChannel, <-eventHorizonChannel)
				}
			}
		}()
	}
}

func Utils() {
	/// reports
	go func() {
		for {
			select {
			case <-endComputing:
				return
			default:
				fmt.Println(bestAgent)
				fmt.Println(runtime.NumGoroutine())
				averageStepAmount := averageStepAmount()
				fmt.Println("averageStepAmount: ", averageStepAmount)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	/// check if got the best answer
	go func() {
		<-maxAccuracy
		fmt.Println("It can't be bether:")
		for i := 0; i < 4*singleServiceAmount+10; i++ {
			endComputing <- true
		}
		fmt.Println(bestAgent)
		averageStepAmount := averageStepAmount()
		fmt.Println("averageStepAmount: ", averageStepAmount)
		exitProgram <- true
		return
	}()
}

func averageStepAmount() uint64 {
	var averageStepAmount uint64
	for i := 0; i < agentAmount; i++ {
		averageStepAmount += agentList[i].times
	}
	averageStepAmount /= uint64(agentAmount)
	return averageStepAmount
}

func init() {
	go func() {
		for {
			randomBuffer <- NextDouble()
		}
	}()
	border.SetUp()

	/// create agents
	for i := 0; i < agentAmount; i++ {
		agent := Agent{
			x:              <-randomBuffer*border.HorizontalLength - border.HorizontalLength/2 + border.HorizontalCenter,
			y:              <-randomBuffer*border.VerticalLength - border.VerticalLength/2 + border.VerticalCenter,
			fitness:        math.MaxFloat64,
			border:         border,
			typeOfFunction: typeOfFunction}
		agentList[i] = &agent
	}

	/// send agents to channel
	for i := 0; i < agentAmount; i++ {
		newAgentChannel <- agentList[i]
	}

	/// set pre-values for agents
	for i := i; i < agentAmount; i++ {
		countFitness(fitnessChannel, <-newAgentChannel)
	}
	/// this goroutine continuously evaluates Best Agent event horizon
	//go func() {
	//	for {
	//		countEventHorizon()
	//	}
	//}()
	bestAgent.eventHorizon = math.SmallestNonzeroFloat64
	var err error
	conn, err = net.Dial("tcp", "172.16.100.11:2110")
	if err != nil {
                log.Println(err)
		os.Exit(-1)

        }

}

func Connect() {
	// waiting for connection:
	c := make(chan string)
	go func() {
		for i := 0; ; i++ {
			c <- fmt.Sprint("asd ", i)
		}
	}()
	for {
		select {
		case s := <-c:
			fmt.Println(s)
		case <-time.After(time.Second * 10):
			fmt.Println("Cannot estabilish a connection")
			return
		default:
			fmt.Println("nothing")

		}
	}
}
