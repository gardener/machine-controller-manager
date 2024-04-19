// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package strings is used to provide some util string functions
package strings

// StringSimilarityRatio is used to find the ratio of similarity
// between two strings using the Levenshtein algorithm below
func StringSimilarityRatio(str1, str2 string) float64 {
	avgLen := float64(len(str1)+len(str2)) / 2
	changes := float64(Levenshtein(str1, str2))
	return (1.0 - changes/avgLen)
}

// Levenshtein is used to compare the number of changes required
// to convert the string from one to another
func Levenshtein(str1, str2 string) int {
	s1len := len(str1)
	s2len := len(str2)
	column := make([]int, len(str1)+1)

	for y := 1; y <= s1len; y++ {
		column[y] = y
	}
	for x := 1; x <= s2len; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= s1len; y++ {
			oldkey := column[y]
			var incr int
			if str1[y-1] != str2[x-1] {
				incr = 1
			}

			column[y] = minimum(column[y]+1, column[y-1]+1, lastkey+incr)
			lastkey = oldkey
		}
	}
	return column[s1len]
}

// minimum finds the minimum among 3 numbers
func minimum(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}
