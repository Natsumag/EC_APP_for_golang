package main

import (
	"fmt"
	"github.com/phpdave11/gofpdf"
	"github.com/phpdave11/gofpdf/contrib/gofpdi"
	"net/http"
	"time"
)

type Order struct {
	ID        int       `json:"id"`
	Quantity  int       `json:"quantity"`
	Amount    int       `json:"amount"`
	Product   string    `json:"product"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (app *application) CreateAndSendInvoice(w http.ResponseWriter, r *http.Request) {
	var order Order

	err := app.readJSON(w, r, &order)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	err = app.createInvoicePDF(order)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	resp.Error = false
	resp.Message = fmt.Sprintf("Invoice %d.pdf created and sent to %s", order.ID, order.Email)

	app.writeJSON(w, http.StatusCreated, resp)
}

func (app *application) createInvoicePDF(order Order) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 13, 10)
	pdf.SetAutoPageBreak(true, 0)

	importer := gofpdi.NewImporter()
	t := importer.ImportPage(pdf, "./pdf-templates/invoice.pdf", 1, "/MediaBox")

	pdf.AddPage()
	importer.UseImportedTemplate(pdf, t, 0, 0, 215, 0)
	pdf.SetXY(10, 50)
	pdf.SetFont("Times", "", 11)
	pdf.CellFormat(100, 10, fmt.Sprintf("%s %s", order.FirstName, order.LastName), "", 0, "L", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(100, 10, order.Email, "", 0, "L", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(100, 10, order.CreatedAt.Format("2006-01-02"), "", 0, "L", false, 0, "")
	pdf.SetXY(60, 100)
	pdf.CellFormat(150, 10, order.Product, "", 0, "L", false, 0, "")
	pdf.SetX(170)
	pdf.CellFormat(20, 10, fmt.Sprintf("%d", order.Quantity), "", 0, "C", false, 0, "")
	pdf.SetX(180)
	pdf.CellFormat(20, 10, fmt.Sprintf("￥%d", order.Amount), "", 0, "R", false, 0, "")

	invoicePath := fmt.Sprintf("./invoices/%d.pdf", order.ID)
	err := pdf.OutputFileAndClose(invoicePath)
	if err != nil {
		return err
	}

	return nil
}
