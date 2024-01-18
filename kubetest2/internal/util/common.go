package util

import (
	"strings"
)

func IsStringInSlice(a string, list []string) bool {
    for _, b := range list {
        if strings.EqualFold(a, b) {
            return true
        }
    }
    return false
}
