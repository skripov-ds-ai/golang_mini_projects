package main

import "fmt"

func getMinSize(a, b []int) int {
	minSize := len(a)
	if minSize > len(b) {
		minSize = len(b)
	}
	return minSize
}

func zipSlice(a, b []int) [][2]int {
	minSize := getMinSize(a, b)
	c := make([][2]int, minSize)
	for i := 0; i < minSize; i++ {
		c[i][0] = a[i]
		c[i][1] = b[i]
	}
	return c
}

func zip(a, b []int) chan [2]int {
	minSize := getMinSize(a, b)
	c := make(chan [2]int)

	go func() {
		defer close(c)
		for i := 0; i < minSize; i++ {
			var x [2]int
			x[0] = a[i]
			x[1] = b[i]
			c <- x
		}
	}()

	return c
}

func getUniversalMinSize(slices ...[]int) int {
	if len(slices) == 0 {
		return 0
	}
	minSize := len(slices[0])
	for i := 1; i < len(slices); i++ {
		if minSize > len(slices[i]) {
			minSize = len(slices[i])
		}
	}
	return minSize
}

func moreZip(slices ...[]int) chan []int {
	minSize := getUniversalMinSize(slices...)
	c := make(chan []int)
	slicesNum := len(slices)

	go func() {
		defer close(c)
		for i := 0; i < minSize; i++ {
			x := make([]int, slicesNum)
			for j := 0; j < slicesNum; j++ {
				x[j] = slices[j][i]
			}
			c <- x
		}
	}()

	return c
}

func main() {
	a := []int{1, 2, 3}
	b := []int{0, 42, 10}

	c := zipSlice(a, b)

	fmt.Println("Slice version of zip function:")
	for _, x := range c {
		fmt.Println(x)
	}

	fmt.Println("\nGenerator version of zip function:")
	for x := range zip(a, b) {
		fmt.Println(x)
	}

	d := []int{-1, -3}
	e := []int{7, 7, 7, 8, 5, 4, 2}
	fmt.Println("\nMore universal generator version of zip:")

	for x := range moreZip(a, b, a, d, e) {
		fmt.Println(x)
	}

}
