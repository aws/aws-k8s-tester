// Package csvutil implements CSV utilities.
package csvutil

import (
	"encoding/csv"
	"os"
)

// Save writes data to CSV.
func Save(header []string, rows [][]string, output string) error {
	f, err := os.OpenFile(output, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		f, err = os.Create(output)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	wr := csv.NewWriter(f)
	if err := wr.Write(header); err != nil {
		return err
	}
	if err := wr.WriteAll(rows); err != nil {
		return err
	}
	wr.Flush()
	return wr.Error()
}
