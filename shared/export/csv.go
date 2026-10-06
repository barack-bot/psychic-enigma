package export

import (
	"encoding/csv"
	"io"
)

func WriteCSV(writer io.Writer, header []string, rows [][]string) error {
	output := csv.NewWriter(writer)
	if err := output.Write(header); err != nil {
		return err
	}
	if err := output.WriteAll(rows); err != nil {
		return err
	}
	output.Flush()
	return output.Error()
}
