package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// сюда писать код

func singleHashToString(s1, s2 string) string {
	return fmt.Sprintf("%s~%s", s1, s2)
}

func interfaceToString(data interface{}) string {
	return fmt.Sprintf("%v", data)
}

func multiHashToString(n int, s string) string {
	return fmt.Sprintf("%d%s", n, s)
}

func processStr(in, out chan interface{}, processor func(string) string) {
	wg := &sync.WaitGroup{}
	for data := range in {
		wg.Add(1)

		s := interfaceToString(data)
		go func(s string) {
			defer wg.Done()
			out <- processor(s)
		}(s)
	}
	wg.Wait()
}

func SingleHash(in, out chan interface{}) {
	m := &sync.Mutex{}

	processStr(in, out, func(s string) string {
		h1 := make(chan string)
		go func() {
			defer close(h1)
			h1 <- DataSignerCrc32(s)
		}()

		h2 := make(chan string)
		go func() {
			defer close(h2)
			m.Lock()
			md5 := DataSignerMd5(s)
			m.Unlock()
			h2 <- DataSignerCrc32(md5)
		}()

		return singleHashToString(<-h1, <-h2)
	})
}

func MultiHash(in, out chan interface{}) {
	m := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	processStr(in, out, func(s string) string {
		hashes := make([]string, 6)

		for th := 0; th < 6; th++ {
			wg.Add(1)
			go func(th int) {
				defer wg.Done()
				s1 := multiHashToString(th, s)
				h := DataSignerCrc32(s1)
				m.Lock()
				hashes[th] = h
				m.Unlock()
			}(th)
		}
		wg.Wait()

		return strings.Join(hashes, "")
	})
}

func CombineResults(in, out chan interface{}) {
	var results []string
	for data := range in {
		s := interfaceToString(data)
		results = append(results, s)
	}
	sort.Strings(results)
	out <- strings.Join(results, "_")
}

func ExecutePipeline(jobs ...job) {
	var in, out chan interface{}
	wg := &sync.WaitGroup{}

	for _, j := range jobs {
		in = out
		out = make(chan interface{}, 100)
		wg.Add(1)
		go func(j job, in, out chan interface{}) {
			defer wg.Done()
			defer close(out)
			j(in, out)
		}(j, in, out)
	}
	wg.Wait()
}

func main() {

}
