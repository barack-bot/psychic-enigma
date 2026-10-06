package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var darajaClient = &http.Client{Timeout: 30 * time.Second}

type MPesaHandler struct{ db *pgxpool.Pool }

type mpesaPushRequest struct {
	StudentID   int64   `json:"student_id"`
	Amount      float64 `json:"amount"`
	PhoneNumber string  `json:"phone_number"`
}

type mpesaCallbackRequest struct {
	CheckoutRequestID string `json:"checkout_request_id"`
	ReceiptNumber     string `json:"receipt_number"`
	Status            string `json:"status"`
}

func NewMPesaHandler(db *pgxpool.Pool) *MPesaHandler { return &MPesaHandler{db: db} }

func (h *MPesaHandler) STKPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req mpesaPushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.StudentID <= 0 || req.Amount <= 0 || strings.TrimSpace(req.PhoneNumber) == "" {
		http.Error(w, "student_id, amount and phone_number are required", http.StatusBadRequest)
		return
	}
	if req.Amount != math.Trunc(req.Amount) || req.Amount < 1 {
		http.Error(w, "amount must be a whole number of KES and at least 1", http.StatusBadRequest)
		return
	}
	if authErr := requireStudentOrStaff(r, h.db, req.StudentID); authErr != nil {
		http.Error(w, authErr.Error(), authErr.status)
		return
	}
	checkoutRequestID, merchantRequestID, err := h.darajaPush(r.Context(), req)
	if err != nil { http.Error(w, err.Error(), http.StatusBadGateway); return }
	if _, err := h.db.Exec(r.Context(), `INSERT INTO mpesa_stk_requests (checkout_request_id, student_id, amount, phone_number, status) VALUES ($1, $2, $3, $4, 'PENDING')`, checkoutRequestID, req.StudentID, req.Amount, req.PhoneNumber); err != nil {
		http.Error(w, "could not create STK request", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{"checkout_request_id": checkoutRequestID, "merchant_request_id": merchantRequestID, "status": "PENDING"})
}

func (h *MPesaHandler) darajaPush(ctx context.Context, req mpesaPushRequest) (string, string, error) {
	baseURL := strings.TrimRight(os.Getenv("MPESA_BASE_URL"), "/"); consumerKey := os.Getenv("MPESA_CONSUMER_KEY"); consumerSecret := os.Getenv("MPESA_CONSUMER_SECRET"); shortcode := os.Getenv("MPESA_SHORTCODE"); passkey := os.Getenv("MPESA_PASSKEY"); callbackURL := os.Getenv("MPESA_CALLBACK_URL")
	if baseURL == "" || consumerKey == "" || consumerSecret == "" || shortcode == "" || passkey == "" || callbackURL == "" { return "", "", fmt.Errorf("M-Pesa Daraja configuration is incomplete") }
	authRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/oauth/v1/generate?grant_type=client_credentials", nil)
	authRequest.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(consumerKey+":"+consumerSecret)))
	authResponse, err := darajaClient.Do(authRequest); if err != nil { return "", "", err }; defer authResponse.Body.Close(); if authResponse.StatusCode/100 != 2 { return "", "", fmt.Errorf("Daraja OAuth failed with status %d", authResponse.StatusCode) }
	var authBody struct{ AccessToken string `json:"access_token"` }; if err := json.NewDecoder(authResponse.Body).Decode(&authBody); err != nil || authBody.AccessToken == "" { return "", "", fmt.Errorf("invalid Daraja OAuth response") }
	timestamp := time.Now().Format("20060102150405"); password := base64.StdEncoding.EncodeToString([]byte(shortcode+passkey+timestamp)); payload := map[string]any{"BusinessShortCode": shortcode, "Password": password, "Timestamp": timestamp, "TransactionType": "CustomerPayBillOnline", "Amount": req.Amount, "PartyA": req.PhoneNumber, "PartyB": shortcode, "PhoneNumber": req.PhoneNumber, "CallBackURL": callbackURL, "AccountReference": fmt.Sprintf("STUDENT-%d", req.StudentID), "TransactionDesc": "School fees"}
	data, _ := json.Marshal(payload); pushRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mpesa/stkpush/v1/processrequest", bytes.NewReader(data)); pushRequest.Header.Set("Authorization", "Bearer "+authBody.AccessToken); pushRequest.Header.Set("Content-Type", "application/json")
	pushResponse, err := darajaClient.Do(pushRequest); if err != nil { return "", "", err }; defer pushResponse.Body.Close(); if pushResponse.StatusCode/100 != 2 { return "", "", fmt.Errorf("Daraja STK push failed with status %d", pushResponse.StatusCode) }
	var pushBody struct{ MerchantRequestID string `json:"MerchantRequestID"`; CheckoutRequestID string `json:"CheckoutRequestID"`; ResponseCode string `json:"ResponseCode"`; ResponseDescription string `json:"ResponseDescription"` }; if err := json.NewDecoder(pushResponse.Body).Decode(&pushBody); err != nil || pushBody.CheckoutRequestID == "" || pushBody.ResponseCode != "0" { return "", "", fmt.Errorf("Daraja rejected STK push: %s", pushBody.ResponseDescription) }
	return pushBody.CheckoutRequestID, pushBody.MerchantRequestID, nil
}

func (h *MPesaHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	callbackSecret := strings.TrimSpace(os.Getenv("MPESA_CALLBACK_SECRET"))
	if callbackSecret == "" || r.Header.Get("X-Mpesa-Callback-Secret") != callbackSecret {
		http.Error(w, "callback authentication failed", http.StatusUnauthorized)
		return
	}
	if signatureSecret := os.Getenv("MPESA_CALLBACK_HMAC_SECRET"); signatureSecret != "" { body, err := io.ReadAll(r.Body); if err != nil { http.Error(w, "invalid callback", 400); return }; mac := hmac.New(sha256.New, []byte(signatureSecret)); mac.Write(body); expected := hex.EncodeToString(mac.Sum(nil)); if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Mpesa-Signature"))) { http.Error(w, "callback signature invalid", 401); return }; r.Body = io.NopCloser(bytes.NewReader(body)) }
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid callback", http.StatusBadRequest)
		return
	}
	var payload struct {
		Body struct {
			StkCallback struct {
				MerchantRequestID string `json:"MerchantRequestID"`
				CheckoutRequestID string `json:"CheckoutRequestID"`
				ResultCode        int    `json:"ResultCode"`
				ResultDesc        string `json:"ResultDesc"`
				CallbackMetadata  struct {
					Item []struct {
						Name  string `json:"Name"`
						Value string `json:"Value"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	checkoutRequestID := strings.TrimSpace(payload.Body.StkCallback.CheckoutRequestID)
	if checkoutRequestID == "" {
		var legacy mpesaCallbackRequest
		if err := json.Unmarshal(body, &legacy); err != nil || strings.TrimSpace(legacy.CheckoutRequestID) == "" {
			http.Error(w, "checkout_request_id is required", http.StatusBadRequest)
			return
		}
		checkoutRequestID = legacy.CheckoutRequestID
	}
	var (
		studentID int64
		amount    float64
	)
	if err := h.db.QueryRow(r.Context(), `SELECT student_id, amount FROM mpesa_stk_requests WHERE checkout_request_id = $1`, checkoutRequestID).Scan(&studentID, &amount); err != nil {
		http.Error(w, "stk request not found", http.StatusNotFound)
		return
	}
	status := "FAILED"
	if payload.Body.StkCallback.ResultCode == 0 {
		status = "COMPLETED"
	}
	receiptNumber := ""
	for _, item := range payload.Body.StkCallback.CallbackMetadata.Item {
		if item.Name == "MpesaReceiptNumber" {
			receiptNumber = item.Value
			break
		}
	}
	if strings.TrimSpace(receiptNumber) == "" {
		receiptNumber = strings.TrimSpace(payload.Body.StkCallback.MerchantRequestID)
	}
	if strings.TrimSpace(receiptNumber) == "" {
		receiptNumber = checkoutRequestID
	}
	if _, err := h.db.Exec(r.Context(), `UPDATE mpesa_stk_requests SET status = $1, receipt_number = $2, completed_at = CURRENT_TIMESTAMP WHERE checkout_request_id = $3`, status, receiptNumber, checkoutRequestID); err != nil {
		http.Error(w, "could not update mpesa request", http.StatusInternalServerError)
		return
	}
	if status == "COMPLETED" {
		if _, err := h.db.Exec(r.Context(), `INSERT INTO tuition_payments (student_id, academic_term_id, source, method, amount, transaction_reference, status) VALUES ($1, (SELECT id FROM academic_terms WHERE CURRENT_DATE BETWEEN starts_on AND ends_on ORDER BY starts_on DESC LIMIT 1), 'STUDENT', 'MPESA', $2, $3, 'COMPLETED') ON CONFLICT (transaction_reference) DO NOTHING`, studentID, amount, receiptNumber); err != nil {
			http.Error(w, "could not create payment record", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"checkout_request_id": checkoutRequestID, "status": status, "receipt_number": receiptNumber, "student_id": studentID})
}
