package utils

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ReceiptData struct {
	AssetName             string
	AssetType             string
	SerialNumber          string
	Category              string
	AssigneeName          string
	AssigneeEmail         string
	AssigneePhone         string
	LentAt                time.Time
	SignedAt              time.Time
	SignatureData         string // base64 PNG (may include "data:image/png;base64," prefix)
	ApprovalSignatureData string
	Purpose               string
}

// GenerateHandoverReceipt creates the PDF and returns the file path.
func GenerateHandoverReceipt(data ReceiptData, formUUID string) (string, error) {
	caser := cases.Title(language.English)
	dir := filepath.Join("uploads", "receipts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 12, "ASSET HANDOVER RECEIPT", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 7, "(Surat Serah Terima Aset)", "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Divider
	pdf.SetDrawColor(30, 64, 175)
	pdf.SetLineWidth(0.8)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(5)

	// Body fields helper
	line := func(label, value string) {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(55, 7, label, "", 0, "", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(0, 7, ": "+value, "", 1, "", false, 0, "")
	}

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 9, "Asset Details", "", 1, "", false, 0, "")
	line("Asset Name", data.AssetName)
	line("Asset Type", caser.String(data.AssetType))
	line("Serial Number", data.SerialNumber)
	line("Category", data.Category)
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 9, "Assignee Details", "", 1, "", false, 0, "")
	line("Name", data.AssigneeName)
	line("Email", data.AssigneeEmail)
	line("Phone", data.AssigneePhone)
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 9, "Lending Details", "", 1, "", false, 0, "")
	line("Lent At", data.LentAt.Format("02 January 2006"))
	line("Signed At", data.SignedAt.Format("02 January 2006 15:04"))
	line("Purpose", data.Purpose)
	pdf.Ln(8)

	// Signature section
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 9, "Signatures", "", 1, "", false, 0, "")
	pdf.Ln(2)

	sigY := pdf.GetY()
	const sigW, sigH = 80.0, 30.0
	const leftX, rightX = 15.0, 110.0

	// ── helper: embed or draw a placeholder box ──────────────────────────
	embedSig := func(b64data string, x, y float64, tmpName string) {
		embedded := false
		if b64data != "" {
			raw := b64data
			if idx := strings.Index(raw, ","); idx != -1 {
				raw = raw[idx+1:]
			}
			imgBytes, decErr := base64.StdEncoding.DecodeString(raw)
			if decErr == nil {
				tmpFile := filepath.Join(dir, tmpName)
				if writeErr := os.WriteFile(tmpFile, imgBytes, 0644); writeErr == nil {
					defer os.Remove(tmpFile)
					pdf.ImageOptions(tmpFile, x, y, sigW, sigH, false,
						fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
					embedded = true
				}
			}
		}
		if !embedded {
			pdf.SetDrawColor(200, 200, 200)
			pdf.Rect(x, y, sigW, sigH, "D")
			pdf.SetFont("Arial", "I", 8)
			pdf.SetXY(x, y+sigH/2-3.5)
			pdf.CellFormat(sigW, 7, "[Digital Signature]", "", 0, "C", false, 0, "")
		}
	}

	// Labels above each box
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(leftX, sigY)
	pdf.CellFormat(sigW, 7, "Assignee Signature:", "", 0, "", false, 0, "")
	pdf.SetXY(rightX, sigY)
	pdf.CellFormat(sigW, 7, "Approver Signature:", "", 1, "", false, 0, "")

	sigBoxY := sigY + 8
	embedSig(data.SignatureData, leftX, sigBoxY, formUUID+"_sig.png")
	embedSig(data.ApprovalSignatureData, rightX, sigBoxY, formUUID+"_approval_sig.png")

	// Advance cursor past both boxes
	pdf.SetXY(leftX, sigBoxY+sigH+8)

	pdf.Ln(8)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(0, 5, fmt.Sprintf("Document ID: %s", formUUID), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 5, "This document was digitally signed via the Asset Management System (ISAM).", "", 1, "", false, 0, "")

	outPath := filepath.Join(dir, formUUID+".pdf")
	if err := pdf.OutputFileAndClose(outPath); err != nil {
		return "", fmt.Errorf("pdf output: %w", err)
	}
	return outPath, nil
}
