// Package textract pulls plain text out of a résumé or cover letter, so a
// tailored version can be compared with the default. Layout is lost; the
// words and their order are what's compared.
package textract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

var ErrUnsupported = errors.New("can't read text from this kind of file")

const docxType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// Text returns the file's text, by content type, falling back to the name's extension.
func Text(contentType, filename string, raw []byte) (string, error) {
	ct := strings.ToLower(contentType)
	ext := strings.ToLower(filename[strings.LastIndex(filename, ".")+1:])
	switch {
	case strings.HasPrefix(ct, "text/") || ext == "md" || ext == "txt":
		return string(raw), nil
	case ct == "application/pdf" || ext == "pdf":
		return pdfText(raw)
	case ct == docxType || ext == "docx":
		return docxText(raw)
	}
	return "", ErrUnsupported
}

func pdfText(raw []byte) (out string, err error) {
	// The parser panics on some malformed files.
	defer func() {
		if r := recover(); r != nil {
			out, err = "", errors.New("couldn't read this PDF")
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		rows, err := r.Page(i).GetTextByRow()
		if err != nil {
			return "", err
		}
		for _, row := range rows {
			var line strings.Builder
			for _, w := range row.Content {
				line.WriteString(w.S)
			}
			if s := strings.TrimSpace(line.String()); s != "" {
				b.WriteString(s + "\n")
			}
		}
	}
	return b.String(), nil
}

// docxText reads word/document.xml: each paragraph (w:p) a line, its runs' text (w:t) joined.
func docxText(raw []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		dec := xml.NewDecoder(io.LimitReader(rc, 20<<20))
		var b strings.Builder
		inText := false
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				return b.String(), nil
			}
			if err != nil {
				return "", err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				inText = t.Name.Local == "t"
				if t.Name.Local == "tab" {
					b.WriteString("\t")
				}
			case xml.EndElement:
				inText = false
				if t.Name.Local == "p" {
					b.WriteString("\n")
				}
			case xml.CharData:
				if inText {
					b.Write(t)
				}
			}
		}
	}
	return "", errors.New("no word/document.xml in this file")
}
