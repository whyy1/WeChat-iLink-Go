package main

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const quietZone = 4

func printQRCode(content string) error {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("generate qr code: %w", err)
	}

	size := qr.Bitmap()
	rows := len(size)
	cols := len(size[0])

	totalRows := rows + 2*quietZone
	totalCols := cols + 2*quietZone

	var sb strings.Builder
	for y := 0; y < totalRows; y += 2 {
		for x := 0; x < totalCols; x++ {
			top := isBlack(x, y, size, rows, cols)
			bottom := isBlack(x, y+1, size, rows, cols)

			switch {
			case top && bottom:
				sb.WriteRune('█')
			case top && !bottom:
				sb.WriteRune('▀')
			case !top && bottom:
				sb.WriteRune('▄')
			default:
				sb.WriteRune(' ')
			}
		}
		sb.WriteRune('\n')
	}

	fmt.Print(sb.String())
	return nil
}

func isBlack(x, y int, size [][]bool, rows, cols int) bool {
	ax := x - quietZone
	ay := y - quietZone
	if ax < 0 || ay < 0 || ax >= cols || ay >= rows {
		return false
	}
	return size[ay][ax]
}
