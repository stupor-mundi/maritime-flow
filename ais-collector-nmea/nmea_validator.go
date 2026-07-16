package main

import "strconv"

func ValidateNmea(sentence string) bool {
	if len(sentence) == 0 {
		return false
	}
	if sentence[0] != '!' && sentence[0] != '$' {
		return false
	}

	starIdx := -1
	for i := len(sentence) - 1; i > 0; i-- {
		if sentence[i] == '*' {
			starIdx = i
			break
		}
	}
	if starIdx < 1 || starIdx+3 > len(sentence) {
		return false
	}

	checksumHex := sentence[starIdx+1 : starIdx+3]
	actual, err := strconv.ParseUint(checksumHex, 16, 8)
	if err != nil {
		return false
	}

	var expected byte
	for i := 1; i < starIdx; i++ {
		expected ^= sentence[i]
	}

	return byte(actual) == expected
}
