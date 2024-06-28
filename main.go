package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

const sizeOfBuffer int = 20
const bufferClearInterval time.Duration = time.Second * 15

type ringBuffer struct {
	data    []int
	size    int
	pointer int
	m       sync.Mutex
}

func newBuffer(size int) *ringBuffer {
	return &ringBuffer{make([]int, size), size, 0, sync.Mutex{}}
}

func (r *ringBuffer) push(number int) {
	r.m.Lock()
	defer r.m.Unlock()
	if r.pointer == r.size {
		for i := 1; i < r.size-1; i++ {
			r.data[i-1] = r.data[i]
		}
		r.data[r.pointer] = number
		r.pointer = 0
	} else {
		r.data[r.pointer] = number
		r.pointer++
	}
}

func (r *ringBuffer) get() []int {
	if r.pointer == 0 {
		return nil
	}

	r.m.Lock()
	defer r.m.Unlock()
	output := r.data[:r.pointer]
	r.pointer = 0
	return output
}

type stage func(<-chan bool, <-chan int) <-chan int

type pipeline struct {
	stages []stage
	done   <-chan bool
}

func newPipeline(done <-chan bool, stages ...stage) *pipeline {
	return &pipeline{stages, done}
}

func (p *pipeline) run(integers <-chan int) <-chan int {
	var data <-chan int = integers
	for index := range p.stages {
		data = p.runStage(p.stages[index], data)
	}
	return data
}

func (p *pipeline) runStage(stage stage, integers <-chan int) <-chan int {
	return stage(p.done, integers)
}

func main() {
	init := func() (<-chan int, <-chan bool) {
		done := make(chan bool)
		integers := make(chan int)

		go func() {
			defer close(done)
			scanner := bufio.NewScanner(os.Stdin)
			var data string
			for {
				scanner.Scan()
				data = scanner.Text()
				i, err := strconv.Atoi(data)
				if err != nil {
					log.Println("Пайплайн работает только с целыми числами!")
					continue
				} else {
					log.Println("Число", i, "целое, передано дальше")
				}

				integers <- i
			}
		}()
		return integers, done
	}

	negativeFilter := func(done <-chan bool, integers <-chan int) <-chan int {
		filtratedIntegers := make(chan int)
		go func() {
			for {
				select {
				case number := <-integers:
					if number > 0 {
						select {
						case filtratedIntegers <- number:
							log.Println("Число", number, "положительное, передано дальше")
						case <-done:
							return
						}
					} else {
						log.Println("Число", number, "не положительное")
					}
				case <-done:
					return
				}
			}
		}()
		return filtratedIntegers
	}

	notMultipleOfThreeFilter := func(done <-chan bool, integers <-chan int) <-chan int {
		filtratedIntegers := make(chan int)
		go func() {
			for {
				select {
				case number := <-integers:
					if number > 0 && number%3 == 0 {
						select {
						case filtratedIntegers <- number:
							log.Println("Число", number, "делится на 3, передано дальше")
						case <-done:
							return
						}
					} else {
						log.Println("Число", number, "не делится на 3")
					}
				case <-done:
					return
				}
			}
		}()
		return filtratedIntegers
	}

	bufferStage := func(done <-chan bool, integers <-chan int) <-chan int {
		bufferedIntegers := make(chan int)
		buffer := newBuffer(sizeOfBuffer)
		go func() {
			for {
				select {
				case number := <-integers:
					buffer.push(number)
				case <-done:
					return
				}
			}
		}()

		go func() {
			for {
				select {
				case <-time.After(bufferClearInterval):
					bufferData := buffer.get()
					if bufferData != nil {
						for _, data := range bufferData {
							select {
							case bufferedIntegers <- data:
							case <-done:
								return
							}
						}
					}
				case <-done:
					return
				}
			}
		}()
		return bufferedIntegers
	}

	consumer := func(done <-chan bool, integers <-chan int) {
		for {
			select {
			case number := <-integers:
				log.Println("Получено число: ", number)
			case <-done:
				return
			}
		}
	}

	integers, done := init()

	pipeline := newPipeline(done, negativeFilter, notMultipleOfThreeFilter, bufferStage)
	consumer(done, pipeline.run(integers))
}
