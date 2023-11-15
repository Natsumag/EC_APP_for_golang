package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v75"
	"golang.org/x/crypto/bcrypt"
	"myapp/internal/cards"
	config2 "myapp/internal/config"
	"myapp/internal/encription"
	"myapp/internal/models"
	"myapp/internal/urlsinger"
	"myapp/internal/util"
	"myapp/internal/validator"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type stripePayload struct {
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Email         string `json:"email"`
	CardBrand     string `json:"card_brand"`
	ExpiryMonth   int    `json:"exp_month"`
	ExpiryYear    int    `json:"exp_year"`
	LastFour      string `json:"last_four"`
	Plan          string `json:"plan"`
	ProductID     string `json:"product_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
}

type jsonResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Content string `json:"content,omitempty"`
	ID      int    `json:"id,omitempty"`
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

func (app *application) GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload stripePayload

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	amount, err := strconv.Atoi(payload.Amount)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: payload.Currency,
	}

	okey := true

	pi, msg, err := card.Charge(payload.Currency, amount)
	if err != nil {
		okey = false
	}

	if okey {
		out, err := json.MarshalIndent(pi, "", "	")
		if err != nil {
			app.errorLog.Println(err)
			return
		}
		w.Header().Set("Content-Type", "application.json")
		w.Write(out)
	} else {
		j := jsonResponse{
			OK:      false,
			Message: msg,
			Content: "",
		}

		out, err := json.MarshalIndent(j, "", "	")
		if err != nil {
			app.errorLog.Println(err)
		}
		w.Header().Set("Content-Type", "application.json")
		w.Write(out)
	}
}

func (app *application) GetWidgetByID(w http.ResponseWriter, r *http.Request) {
	widgetID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	widget, err := app.DB.GetWidget(widgetID)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	out, err := json.MarshalIndent(widget, "", "	")
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	w.Header().Set("Content-Type", "application.json")
	w.Write(out)
}

func (app *application) CreateCustomerAndSubscribeToPlan(w http.ResponseWriter, r *http.Request) {
	var data stripePayload
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	// validation
	valid := validator.New()
	nameRegex := regexp.MustCompile("^[a-zA-Z]+$")
	EmailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	// first_name
	valid.Check(len(data.FirstName) > 1 && len(data.FirstName) < 256, "first_name", "must be at least 2 and at most 255 characters")
	valid.Check(nameRegex.MatchString(data.FirstName), "first_name", "invalid input")
	// last_name
	valid.Check(len(data.LastName) > 1 && len(data.LastName) < 256, "last_name", "must be at least 2 and at most 255 characters")
	valid.Check(nameRegex.MatchString(data.LastName), "last_name", "invalid input")
	// email
	valid.Check(len(data.Email) > 8 || len(data.Email) < 320, "email", "must be at least 17 and at most 320 characters")
	valid.Check(EmailRegex.MatchString(data.Email), "email", "invalid input")
	domain, _ := util.ExtractDomain(data.Email)
	if _, err := net.LookupMX(domain); err != nil {
		valid.Check(false, "email", "not exist domain")
	}

	if !valid.Valid() {
		app.failedValidation(w, r, valid.Errors)
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: data.Currency,
	}

	okey := true
	var subscription *stripe.Subscription
	txnMsg := "Transaction successful"

	stripeCustomer, msg, err := card.CreateCustomer(data.PaymentMethod, data.Email)
	if err != nil {
		app.errorLog.Println(err)
		okey = false
		txnMsg = msg
	}

	if okey {
		subscription, err = card.SubscribeToPlan(stripeCustomer, data.Plan, data.Email, data.LastFour, "")
		if err != nil {
			app.errorLog.Println(err)
			okey = false
			txnMsg = "Error subscribing customer"
		}

		app.infoLog.Println("subscription id:", subscription.ID)
	}

	if okey {
		config := config2.LoadConfig()
		productID, _ := strconv.Atoi(data.ProductID)
		customerID, err := app.SaveCustomer(data.FirstName, data.LastName, data.Email)
		if err != nil {
			app.errorLog.Println(err)
			return
		}

		amount, _ := strconv.Atoi(data.Amount)
		txn := models.Transaction{
			Amount:              amount,
			Currency:            "cad",
			LastFour:            data.LastFour,
			ExpiryMonth:         data.ExpiryMonth,
			ExpiryYear:          data.ExpiryYear,
			TransactionStatusID: config.Status["Refunded"],
			PaymentIntent:       subscription.ID,
			PaymentMethod:       data.PaymentMethod,
		}
		txnID, err := app.SaveTransaction(txn)
		if err != nil {
			app.errorLog.Println(err)
			return
		}

		order := models.Order{
			WidgetID:      productID,
			TransactionID: txnID,
			CustomerID:    customerID,
			StatusID:      config.Status["Cleared"],
			Quantity:      1,
			Amount:        amount,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		orderID, err := app.SaveOrder(order)
		if err != nil {
			app.errorLog.Println(err)
			return
		}

		invoice := Invoice{
			ID:        orderID,
			Amount:    2000,
			Product:   "Bronze plan",
			Quantity:  order.Quantity,
			FirstName: data.FirstName,
			LastName:  data.LastName,
			Email:     data.Email,
			CreatedAt: time.Now(),
		}
		err = app.callInvoiceMicro(invoice)
		if err != nil {
			app.errorLog.Println(err)
			return
		}
	}

	resp := jsonResponse{
		OK:      okey,
		Message: txnMsg,
	}
	out, err := json.MarshalIndent(resp, "", "	")
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (app *application) callInvoiceMicro(invoice Invoice) error {
	url := app.config.microurl + "/invoice/create-and-send"
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

	return nil
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

func (app *application) CreateAuthToken(w http.ResponseWriter, r *http.Request) {
	var userInput struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := app.readJSON(w, r, &userInput)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// validation
	valid := validator.New()
	EmailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	// email
	valid.Check(len(userInput.Email) > 8 || len(userInput.Email) < 320, "email", "must be at least 17 and at most 320 characters")
	valid.Check(EmailRegex.MatchString(userInput.Email), "email", "invalid input")
	domain, _ := util.ExtractDomain(userInput.Email)
	if _, err := net.LookupMX(domain); err != nil {
		valid.Check(false, "email", "not exist domain")
	}
	// password
	valid.Check(len(userInput.Password) > 8 || len(userInput.Password) < 256, "password", "must be at least 17 and at most 256 characters")
	if !valid.Valid() {
		app.failedValidation(w, r, valid.Errors)
		return
	}

	user, err := app.DB.GetUserByEmail(userInput.Email)
	if err != nil {
		app.invalidCredentials(w)
		return
	}

	validPassword, err := app.passwordMatches(user.Password, userInput.Password)
	if err != nil {
		app.invalidCredentials(w)
		return
	}

	if !validPassword {
		app.invalidCredentials(w)
		return
	}

	token, err := models.GenerateToken(user.ID, 24*time.Hour, models.ScopeAuthentication)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	err = app.DB.InsertToken(token, user)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var payload struct {
		Error   bool          `json:"error"`
		Message string        `json:"message"`
		Token   *models.Token `json:"authentication_token"`
	}
	payload.Error = false
	payload.Message = fmt.Sprintf("token for %s created", userInput.Email)
	payload.Token = token

	_ = app.writeJSON(w, http.StatusOK, payload)
}

func (app *application) CreateAuthentication(w http.ResponseWriter, r *http.Request) {
	user, err := app.authenticateToken(r)
	if err != nil {
		app.invalidCredentials(w)
		return
	}

	var payload struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	payload.Error = false
	payload.Message = fmt.Sprintf("authenticated user %s", user.Email)
	app.writeJSON(w, http.StatusOK, payload)
}

func (app *application) authenticateToken(r *http.Request) (*models.User, error) {
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader == "" {
		return nil, errors.New("no authorization header received")
	}

	headerParts := strings.Split(authorizationHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return nil, errors.New("no authorization header received")
	}

	token := headerParts[1]
	if len(token) != 26 {
		return nil, errors.New("authentication token wrong size")
	}

	user, err := app.DB.GetUserByToken(token)
	if err != nil {
		return nil, errors.New("no matching user found")
	}

	return user, nil
}

func (app *application) VirtualTerminalPaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	var txnData struct {
		PaymentAmount   int    `json:"amount"`
		PaymentCurrency string `json:"currency"`
		FirstName       string `json:"first_name"`
		LastName        string `json:"last_name"`
		Email           string `json:"email"`
		PaymentIntent   string `json:"payment_intent"`
		PaymentMethod   string `json:"payment_method"`
		BankReturnCode  string `json:"bank_return_code"`
		ExpiryMonth     int    `json:"expiry_month"`
		ExpiryYear      int    `json:"expiry_year"`
		LastFour        string `json:"last_four"`
	}

	err := app.readJSON(w, r, &txnData)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	card := cards.Card{
		Secret: app.config.stripe.secret,
		Key:    app.config.stripe.key,
	}

	pi, err := card.RetrievePaymentIntent(txnData.PaymentIntent)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	pm, err := card.GetPaymentMethod(txnData.PaymentMethod)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	txnData.LastFour = pm.Card.Last4
	txnData.ExpiryMonth = int(pm.Card.ExpMonth)
	txnData.ExpiryYear = int(pm.Card.ExpYear)
	config := config2.LoadConfig()

	txn := models.Transaction{
		Amount:              txnData.PaymentAmount,
		Currency:            txnData.PaymentCurrency,
		LastFour:            txnData.LastFour,
		ExpiryMonth:         txnData.ExpiryMonth,
		ExpiryYear:          txnData.ExpiryYear,
		PaymentIntent:       txnData.PaymentIntent,
		PaymentMethod:       txnData.PaymentMethod,
		BankReturnCode:      pi.LatestCharge.ID,
		TransactionStatusID: config.Status["Refunded"],
	}

	_, err = app.SaveTransaction(txn)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, txn)
}

func (app *application) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email string `json:"email"`
	}

	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// validation
	valid := validator.New()
	EmailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	// email
	valid.Check(len(payload.Email) > 8 || len(payload.Email) < 320, "email", "must be at least 17 and at most 320 characters")
	valid.Check(EmailRegex.MatchString(payload.Email), "email", "invalid input")
	domain, _ := util.ExtractDomain(payload.Email)
	if _, err := net.LookupMX(domain); err != nil {
		valid.Check(false, "email", "not exist domain")
	}

	if !valid.Valid() {
		app.failedValidation(w, r, valid.Errors)
		return
	}

	_, err = app.DB.GetUserByEmail(payload.Email)
	app.infoLog.Println("err", err)
	if err != nil {
		var resp struct {
			Error   bool   `json:"error"`
			Message string `json:"message"`
		}
		resp.Error = true
		resp.Message = "no matching email found"
		app.writeJSON(w, http.StatusAccepted, resp)
		return
	}

	link := fmt.Sprintf("%s/reset-password?email=%s", app.config.weburl, payload.Email)
	sign := urlsinger.Singer{
		Secret: []byte(app.config.secretkey),
	}

	signedLink := sign.GenerateTokenFromString(link)

	var data struct {
		Link string
	}

	data.Link = signedLink
	err = app.SendMail(app.config.smtp.frommail, payload.Email, "Password reset request", "password-reset", data)
	if err != nil {
		app.errorLog.Println(data)
		app.badRequest(w, r, err)
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}

	resp.Error = false

	app.writeJSON(w, http.StatusCreated, resp)
}

func (app *application) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	encryptor := encription.Encryption{
		Key: []byte(app.config.secretkey),
	}

	email, err := encryptor.Decrypt(payload.Email)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	user, err := app.DB.GetUserByEmail(email)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), 12)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	err = app.DB.UpdatePasswordForUser(user, string(newHash))
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	resp.Error = false
	resp.Message = "password change"

	app.writeJSON(w, http.StatusCreated, resp)
}

func (app *application) AllSales(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PageSize    int `json:"page_size"`
		CurrentPage int `json:"current_page"`
	}

	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	config := config2.LoadConfig()
	isRecurring := config.IsRecurring["NoRecurring"]
	allSales, totalRecords, lastPage, err := app.DB.GetAllOrdersPaginated(isRecurring, payload.PageSize, payload.CurrentPage)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var resp struct {
		Orders       []*models.Order `json:"orders"`
		CurrentPage  int             `json:"current_page"`
		PageSize     int             `json:"page_size"`
		TotalRecords int             `json:"total_records"`
		LastPage     int             `json:"last_page"`
	}
	resp.Orders = allSales
	resp.CurrentPage = payload.CurrentPage
	resp.PageSize = payload.PageSize
	resp.TotalRecords = totalRecords
	resp.LastPage = lastPage

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *application) GetSale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orderID, _ := strconv.Atoi(id)
	order, err := app.DB.GetOrderByID(orderID)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, order)
}

func (app *application) AllSubscriptions(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PageSize    int `json:"page_size"`
		CurrentPage int `json:"current_page"`
	}
	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	config := config2.LoadConfig()
	isRecurring := config.IsRecurring["Recurring"]
	allSubscriptions, totalRecords, lastPage, err := app.DB.GetAllOrdersPaginated(isRecurring, payload.PageSize, payload.CurrentPage)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var resp struct {
		Orders       []*models.Order `json:"orders"`
		CurrentPage  int             `json:"current_page"`
		PageSize     int             `json:"page_size"`
		TotalRecords int             `json:"total_records"`
		LastPage     int             `json:"last_page"`
	}
	resp.Orders = allSubscriptions
	resp.CurrentPage = payload.CurrentPage
	resp.PageSize = payload.PageSize
	resp.TotalRecords = totalRecords
	resp.LastPage = lastPage

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *application) RefundCharge(w http.ResponseWriter, r *http.Request) {
	var chargeToRefund struct {
		ID            int    `json:"id"`
		PaymentIntent string `json:"pi"`
		Amount        int    `json:"amount"`
		Currency      string `json:"currency"`
	}

	err := app.readJSON(w, r, &chargeToRefund)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: chargeToRefund.Currency,
	}

	config := config2.LoadConfig()
	err = app.DB.UpdateOrderStatus(chargeToRefund.ID, config.Status["Refunded"])
	if err != nil {
		app.badRequest(w, r, errors.New("charge was refunded"))
		return
	}

	err = card.Refund(chargeToRefund.PaymentIntent, chargeToRefund.Amount)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	resp.Error = false
	resp.Message = "charge refund"

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *application) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	var subToCancel struct {
		ID            int    `json:"id"`
		PaymentIntent string `json:"pi"`
		Currency      string `json:"currency"`
	}

	err := app.readJSON(w, r, &subToCancel)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: subToCancel.Currency,
	}

	err = card.CancelSubscription(subToCancel.PaymentIntent)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	config := config2.LoadConfig()
	err = app.DB.UpdateOrderStatus(subToCancel.ID, config.Status["Cancelled"])
	if err != nil {
		app.badRequest(w, r, errors.New("charge was cancelled"))
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	resp.Error = false
	resp.Message = "subscription cancelled"

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *application) AllUsers(w http.ResponseWriter, r *http.Request) {
	allUsers, err := app.DB.GetAllUsers()
	if err != nil {
		app.badRequest(w, r, errors.New("charge was cancelled"))
		return
	}

	app.writeJSON(w, http.StatusOK, allUsers)
}

func (app *application) OneUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		app.badRequest(w, r, errors.New("invalid url"))
		return
	}

	user, err := app.DB.GetUser(userID)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, user)
}

func (app *application) EditUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		app.badRequest(w, r, errors.New("invalid url"))
		return
	}

	var user models.User
	err = app.readJSON(w, r, &user)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// validation
	valid := validator.New()
	nameRegex := regexp.MustCompile("^[a-zA-Z]+$")
	EmailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	// first_name
	valid.Check(len(user.FirstName) > 1 && len(user.FirstName) < 256, "first_name", "must be at least 2 and at most 255 characters")
	valid.Check(nameRegex.MatchString(user.FirstName), "first_name", "invalid input")
	// last_name
	valid.Check(len(user.LastName) > 1 && len(user.LastName) < 256, "last_name", "must be at least 2 and at most 255 characters")
	valid.Check(nameRegex.MatchString(user.LastName), "last_name", "invalid input")
	// email
	valid.Check(len(user.Email) > 8 || len(user.Email) < 320, "email", "must be at least 17 and at most 320 characters")
	valid.Check(EmailRegex.MatchString(user.Email), "email", "invalid input")
	domain, _ := util.ExtractDomain(user.Email)
	if _, err := net.LookupMX(domain); err != nil {
		valid.Check(false, "email", "not exist domain")
	}
	// password
	valid.Check(len(user.Password) > 8 || len(user.Password) < 256, "password", "must be at least 17 and at most 256 characters")
	valid.Check(user.Password == user.VerifyPassword, "password", "no matching password and verify password")

	if !valid.Valid() {
		app.failedValidation(w, r, valid.Errors)
		return
	}

	if userID > 0 {
		err = app.DB.EditUser(user)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		if user.Password != "" {
			newHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
			if err != nil {
				app.badRequest(w, r, err)
				return
			}

			err = app.DB.UpdatePasswordForUser(user, string(newHash))
			if err != nil {
				app.badRequest(w, r, err)
				return
			}
		}
	} else {
		newHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 12)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		err = app.DB.AddUser(user, string(newHash))
		if err != nil {
			app.badRequest(w, r, err)
			return
		}
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	resp.Error = false
	resp.Message = "user edited"

	app.writeJSON(w, http.StatusOK, resp)
}

func (app *application) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		app.badRequest(w, r, errors.New("invalid url"))
		return
	}
	err = app.DB.DeleteUser(userID)
	if err != nil {
		app.badRequest(w, r, errors.New("invalid url"))
		return
	}

	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	resp.Error = false
	resp.Message = "user edited"

	app.writeJSON(w, http.StatusOK, resp)
}
