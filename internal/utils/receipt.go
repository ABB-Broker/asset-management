package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-pdf/fpdf"
)

type ReceiptData struct {
	AssetName     string
	AssetType     string
	SerialNumber  string
	Category      string
	AssigneeName  string
	AssigneeEmail string
	AssigneePhone string
	LentAt        time.Time
	SignedAt      time.Time
	SignatureData string // base64 PNG (strip the "data:image/png;base64," prefix before embedding)
}

// GenerateHandoverReceipt creates the PDF and returns the file path.
func GenerateHandoverReceipt(data ReceiptData, formUUID string) (string, error) {
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
	line("Asset Type", data.AssetType)
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
	pdf.Ln(8)

	// Signature section
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 7, "Assignee Signature:", "", 1, "", false, 0, "")
	pdf.Ln(2)

	// TODO: Embed the actual signature image.
	// Strip "data:image/png;base64," prefix, decode base64, write to temp file,
	// then use pdf.ImageOptions() to embed it.
	// Example stub:
	pdf.SetDrawColor(200, 200, 200)
	pdf.Rect(15, pdf.GetY(), 80, 30, "D")
	pdf.SetFont("Arial", "I", 8)
	pdf.SetXY(15, pdf.GetY()+12)
	pdf.CellFormat(80, 7, "[Digital Signature]", "", 1, "C", false, 0, "")

	pdf.Ln(35)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(0, 5, fmt.Sprintf("Document ID: %s", formUUID), "", 1, "", false, 0, "")
	pdf.CellFormat(0, 5, "This document was digitally signed via the Asset Management System.", "", 1, "", false, 0, "")

	outPath := filepath.Join(dir, formUUID+".pdf")
	if err := pdf.OutputFileAndClose(outPath); err != nil {
		return "", fmt.Errorf("pdf output: %w", err)
	}
	return outPath, nil
}
