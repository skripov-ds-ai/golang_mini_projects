package main

import "fmt"

func isMonotonic(a []int) (ok bool) {
	if len(a) <= 1 {
		return true
	}
	isGte := a[0] >= a[1]
	isLte := a[0] <= a[1]
	for i := 2; i < len(a); i++ {
		if isGte && a[i-1] < a[i] {
			isGte = false
			if !isLte {
				break
			}
		}
		if isLte && a[i-1] > a[i] {
			isLte = false
			if !isGte {
				break
			}
		}
	}
	return isLte || isGte
}

func main() {
	// true
	a := []int{1, 2, 3}
	// false
	b := []int{3, 2, 3}
	// false
	bb := []int{2, 2, 2, 3, 2}
	// true
	c := []int{3, 2, 1}
	// true
	d := []int{1, 1, 1}
	// true
	e := []int{42}
	// true
	var f []int
	slices := make([][]int, 7)
	slices[0] = a
	slices[1] = b
	slices[2] = bb
	slices[3] = c
	slices[4] = d
	slices[5] = e
	slices[6] = f

	for _, slice := range slices {
		result := isMonotonic(slice)
		fmt.Println(result)
	}
}
