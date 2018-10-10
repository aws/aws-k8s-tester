package accesslog

import (
	"encoding/csv"
	"os"
)

func toCSV(header []string, rows [][]string, p string) error {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		f, err = os.Create(p)
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
