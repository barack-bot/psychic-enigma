package export

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// WriteSimplePDF generates a minimal single-page PDF with plain text lines.
// This is intentionally dependency-free; for rich layout use a PDF library.
func WriteSimplePDF(w io.Writer, title string, lines []string) error {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	content := fmt.Sprintf("BT /F1 14 Tf 50 750 Td (%s) Tj ET\n", escapePDF(title))
	y := 720
	for _, line := range lines {
		content += fmt.Sprintf("BT /F1 10 Tf 50 %d Td (%s) Tj ET\n", y, escapePDF(line))
		y -= 14
		if y < 50 {
			break
		}
	}

	objects := []string{
		"1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n",
		"2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj\n",
		"3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources<< /Font<< /F1 4 0 R >> >> /Contents 5 0 R >>endobj\n",
		"4 0 obj<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>endobj\n",
		fmt.Sprintf("5 0 obj<< /Length %d >>stream\n%s\nendstream endobj\n", len(content), content),
	}

	offset := len(buf.String())
	offsets := []int{0}
	for _, obj := range objects {
		offsets = append(offsets, offset)
		buf.WriteString(obj)
		offset = buf.Len()
	}
	xrefOffset := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(offsets)))
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i < len(offsets); i++ {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	buf.WriteString(fmt.Sprintf("trailer<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(offsets), xrefOffset))

	_, err := w.Write(buf.Bytes())
	return err
}

func escapePDF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}