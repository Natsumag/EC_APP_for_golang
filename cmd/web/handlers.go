package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"myapp/internal/cards"
	"myapp/internal/encription"
	"myapp/internal/models"
	"myapp/internal/urlsinger"
	"net/http"
	"strconv"
	"time"
)

type TransactionData struct {
	FirstName       string
	LastName        string
	Email           string
	PaymentIntentID string
	PaymentMethodID string
	PaymentAmount   int
	PaymentCurrency string
	LastFour        string
	ExpiryMonth     int
	ExpiryYear      int
	BankReturnCode  string
}

type Invoice struct {
	ID        int       `json:"id"`
	Quantity  int       `json:"quantity"`
	Amount    int       `json:"amount"`
	Product   string    `json:"product"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (app *application) Home(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "home", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) VirtualTerminal(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "terminal", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) GetTransactionData(r *http.Request) (TransactionData, error) {
	var txnData TransactionData

	err := r.ParseForm()
	if err != nil {
		app.errorLog.Println(err)
		return txnData, err
	}

	paymentIntent := r.Form.Get("payment_intent")
	paymentMethod := r.Form.Get("payment_method")
	paymentAmount := r.Form.Get("payment_amount")
	amount, _ := strconv.Atoi(paymentAmount)

	card := cards.Card{
		Secret: loadConfig.Stripe.Secret,
		Key:    loadConfig.Stripe.Key,
	}
	pi, err := card.RetrievePaymentIntent(paymentIntent)
	if err != nil {
		app.errorLog.Println(err)
		return txnData, err
	}
	pm, err := card.GetPaymentMethod(paymentMethod)
	if err != nil {
		app.errorLog.Println(err)
		return txnData, err
	}

	txnData = TransactionData{
		FirstName:       r.Form.Get("first_name"),
		LastName:        r.Form.Get("last_name"),
		Email:           r.Form.Get("email"),
		PaymentIntentID: paymentIntent,
		PaymentMethodID: paymentMethod,
		PaymentAmount:   amount,
		PaymentCurrency: r.Form.Get("payment_currency"),
		LastFour:        pm.Card.Last4,
		ExpiryMonth:     int(pm.Card.ExpMonth),
		ExpiryYear:      int(pm.Card.ExpYear),
		BankReturnCode:  pi.LatestCharge.ID,
	}

	return txnData, nil
}

func (app *application) PaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	widgetID, _ := strconv.Atoi(r.Form.Get("product_id"))

	txnData, err := app.GetTransactionData(r)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	// create new customer
	customerID, err := app.SaveCustomer(txnData.FirstName, txnData.LastName, txnData.Email)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	// create new transaction
	txn := models.Transaction{
		Amount:              txnData.PaymentAmount,
		Currency:            txnData.PaymentCurrency,
		LastFour:            txnData.LastFour,
		ExpiryMonth:         txnData.ExpiryMonth,
		ExpiryYear:          txnData.ExpiryYear,
		PaymentIntent:       txnData.PaymentIntentID,
		PaymentMethod:       txnData.PaymentMethodID,
		BankReturnCode:      txnData.BankReturnCode,
		TransactionStatusID: loadConfig.Status["Refunded"],
	}

	txnID, err := app.SaveTransaction(txn)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	// create new order
	order := models.Order{
		WidgetID:      widgetID,
		TransactionID: txnID,
		CustomerID:    customerID,
		StatusID:      loadConfig.Status["Cleared"],
		Quantity:      1,
		Amount:        txnData.PaymentAmount,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	orderID, err := app.SaveOrder(order)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	// call invoice
	invoice := Invoice{
		ID:        orderID,
		Amount:    order.Amount,
		Product:   "Widget",
		Quantity:  order.Quantity,
		FirstName: txnData.FirstName,
		LastName:  txnData.LastName,
		Email:     txnData.Email,
		CreatedAt: time.Now(),
	}
	err = app.callInvoiceMicro(invoice)
	if err != nil {
		app.errorLog.Println(err)
	}

	app.Session.Put(r.Context(), "receipt", txnData)
	http.Redirect(w, r, "/receipt", http.StatusSeeOther)
}

func (app *application) callInvoiceMicro(invoice Invoice) error {
	url := loadConfig.MicroURL + "/invoice/create-and-send"
	out, err := json.MarshalIndent(invoice, "", "\t")
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(out))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	app.infoLog.Println(resp.Body)

	return nil
}

func (app *application) VirtualTerminalPaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	txnData, err := app.GetTransactionData(r)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	// create new transaction
	txn := models.Transaction{
		Amount:              txnData.PaymentAmount,
		Currency:            txnData.PaymentCurrency,
		LastFour:            txnData.LastFour,
		ExpiryMonth:         txnData.ExpiryMonth,
		ExpiryYear:          txnData.ExpiryYear,
		PaymentIntent:       txnData.PaymentIntentID,
		PaymentMethod:       txnData.PaymentMethodID,
		BankReturnCode:      txnData.BankReturnCode,
		TransactionStatusID: loadConfig.Status["Refunded"],
	}

	_, err = app.SaveTransaction(txn)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	app.Session.Put(r.Context(), "receipt", txnData)
	http.Redirect(w, r, "/virtual-terminal-receipt", http.StatusSeeOther)
}

func (app *application) Receipt(w http.ResponseWriter, r *http.Request) {
	txn := app.Session.Get(r.Context(), "receipt").(TransactionData)
	data := make(map[string]interface{})
	data["txn"] = txn
	app.Session.Remove(r.Context(), "receipt")
	if err := app.renderTemplate(w, r, "receipt", &templateData{
		Data: data,
	}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) VirtualTerminalReceipt(w http.ResponseWriter, r *http.Request) {
	txn := app.Session.Get(r.Context(), "receipt").(TransactionData)
	data := make(map[string]interface{})
	data["txn"] = txn
	app.Session.Remove(r.Context(), "receipt")
	if err := app.renderTemplate(w, r, "virtual-terminal-receipt", &templateData{
		Data: data,
	}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) SaveCustomer(firstName, lastName, email string) (int, error) {
	customer := models.Customer{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
	}
	id, err := app.DB.InsertCustomer(customer)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (app *application) SaveTransaction(txn models.Transaction) (int, error) {
	id, err := app.DB.InsertTransaction(txn)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (app *application) SaveOrder(order models.Order) (int, error) {
	id, err := app.DB.InsertOrder(order)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (app *application) ChargeOnce(w http.ResponseWriter, r *http.Request) {
	widgetID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	widget, err := app.DB.GetWidget(widgetID)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	data := make(map[string]interface{})
	data["widget"] = widget

	if err := app.renderTemplate(w, r, "buy-once", &templateData{Data: data}, "stripe-js"); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) BronzePlan(w http.ResponseWriter, r *http.Request) {
	widget, err := app.DB.GetWidget(2)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	data := make(map[string]interface{})
	data["widget"] = widget
	if err := app.renderTemplate(w, r, "bronze-plan", &templateData{
		Data: data,
	}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) BronzePlanReceipt(w http.ResponseWriter, r *http.Request) {

	if err := app.renderTemplate(w, r, "receipt-plan", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) LoginPage(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "login", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) PostLoginPage(w http.ResponseWriter, r *http.Request) {
	app.Session.RenewToken(r.Context())

	err := r.ParseForm()
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	id, err := app.DB.Authenticate(email, password)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.Session.Put(r.Context(), "userID", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *application) Logout(w http.ResponseWriter, r *http.Request) {
	app.Session.Destroy(r.Context())
	app.Session.RenewToken(r.Context())

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *application) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "forgot-password", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) ShowResetPassword(w http.ResponseWriter, r *http.Request) {
	theURL := r.RequestURI
	testURL := fmt.Sprintf("%s%s", loadConfig.WebURL, theURL)

	singer := urlsinger.Singer{
		Secret: []byte(loadConfig.SecretKey),
	}

	valid := singer.VerifyToken(testURL)

	if !valid {
		app.errorLog.Println("invalid url tempering detected")
		return
	}

	expired := singer.Expired(testURL, 60)
	if expired {
		app.errorLog.Println("url expired")
		return
	}

	encryptor := encription.Encryption{
		Key: []byte(loadConfig.SecretKey),
	}
	encryptedEmail, err := encryptor.Encrypt(r.URL.Query().Get("email"))
	if err != nil {
		app.errorLog.Println("encryption failed")
		return
	}

	data := make(map[string]interface{})
	data["email"] = encryptedEmail

	if err := app.renderTemplate(w, r, "reset-password", &templateData{
		Data: data,
	}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) AllSales(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "all-sales", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) ShowSale(w http.ResponseWriter, r *http.Request) {
	stringMap := make(map[string]string)
	stringMap["title"] = "Sale"
	stringMap["cancel"] = "/admin/all-sales"
	stringMap["refund-url"] = "/api/admin/refund"
	stringMap["refund-btn"] = "Refund Order"
	stringMap["refund-badge"] = "Refunded"
	stringMap["refund-msg"] = "Charge refunded"

	if err := app.renderTemplate(w, r, "sale", &templateData{
		StringMap: stringMap,
	}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) AllSubscriptions(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "all-subscriptions", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) ShowSubscription(w http.ResponseWriter, r *http.Request) {
	stringMap := make(map[string]string)
	stringMap["title"] = "Subscription"
	stringMap["cancel"] = "/admin/all-subscriptions"
	stringMap["refund-url"] = "/api/admin/cancel"
	stringMap["refund-btn"] = "Cancel Subscription"
	stringMap["refund-badge"] = "Cancelled"
	stringMap["refund-msg"] = "Subscription cancelled"

	if err := app.renderTemplate(w, r, "sale", &templateData{
		StringMap: stringMap,
	}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) AllUsers(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "all-users", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}

func (app *application) ShowUser(w http.ResponseWriter, r *http.Request) {
	if err := app.renderTemplate(w, r, "one-user", &templateData{}); err != nil {
		app.errorLog.Println(err)
	}
}
