package main

import (
	"fmt"
	"time"
)

// ЗАДАНИЕ:
// * сделать из плохого кода хороший;
// * важно сохранить логику появления ошибочных тасков;
// * сделать правильную мультипоточность обработки заданий.
// Обновленный код отправить через merge-request.

// приложение эмулирует получение и обработку тасков, пытается и получать и обрабатывать в многопоточном режиме
// В конце должно выводить успешные таски и ошибки выполнены остальных тасков

// A Ttype represents a meaninglessness of our life
type Ttype struct {
	id         int
	cT         string // время создания
	fT         string // время выполнения
	taskRESULT []byte
}

func createTasks(a chan Ttype) {
	for {
		ft := time.Now().Format(time.RFC3339)
		if time.Now().Nanosecond()%2 > 0 {
			// вот такое условие появления ошибочных тасков
			ft = "Some error occured"
		}
		// передаем таск на выполнение
		a <- Ttype{cT: ft, id: int(time.Now().Unix())}
	}
}

func doTask(a Ttype) Ttype {
	tt, _ := time.Parse(time.RFC3339, a.cT)
	if tt.After(time.Now().Add(-20 * time.Second)) {
		a.taskRESULT = []byte("task has been successed")
	} else {
		a.taskRESULT = []byte("something went wrong")
	}
	a.fT = time.Now().Format(time.RFC3339Nano)
	time.Sleep(time.Millisecond * 150)
	return a
}

func distributeTask(t Ttype, doneTasks chan<- Ttype, undoneTasks chan<- error) {
	if string(t.taskRESULT[14:]) == "successed" {
		doneTasks <- t
	} else {
		undoneTasks <- fmt.Errorf("Task id %d time %s, error %s", t.id, t.cT, t.taskRESULT)
	}
}

func main() {
	superChan := make(chan Ttype, 10)

	go createTasks(superChan)

	doneTasks := make(chan Ttype)
	undoneTasks := make(chan error)

	go func() {
		// получение тасков
		for t := range superChan {
			t = doTask(t)
			go distributeTask(t, doneTasks, undoneTasks)
		}
		close(superChan)
	}()

	// ошибка: map и slice не защищены мьютексами;
	// потенциальный data race
	// как решить проблему?
	// 1) сделать err/result каналами
	// 2) использовать WaitGroup, чтобы дождаться завершения письма - но таски бесконечно создаются
	// 3) т.к. таски идут бесконечно => будем выводить результаты по приходу по типу "<Done tasks>/<Errors:> <task/error>
	// также ранее была ошибка с захватом переменных
	go func() {
		for done := range doneTasks {
			fmt.Printf("Done tasks: %v\n", done)
		}
	}()
	for undone := range undoneTasks {
		fmt.Printf("Errors: %s\n", undone)
	}
}
