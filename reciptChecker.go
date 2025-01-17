package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	// "github.com/google/uuid"
)

type Item struct {
	ShortDescription string `json:"shortDescription"`
	Price            string `json:"price"`
}

type Receipt struct {
	Retailer     string `json:"retailer"`
	PurchaseDate string `json:"purchaseDate"`
	PurchaseTime string `json:"purchaseTime"`
	Items        []Item `json:"items"`
	Total        string `json:"total"`
}

type ReceiptProcessor struct {
	data map[string]int
	mux  sync.Mutex
}

func NewReceiptProcessor() *ReceiptProcessor {
	return &ReceiptProcessor{
		data: make(map[string]int),
	}
}

func (rp *ReceiptProcessor) ProcessReceipt(w http.ResponseWriter, r *http.Request) {
	var receipt Receipt
	if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	points := rp.calculatePoints(receipt)
	id := uuid.New().String()

	rp.mux.Lock()
	rp.data[id] = points
	rp.mux.Unlock()

	response := map[string]string{"id": id}
	json.NewEncoder(w).Encode(response)
}

func (rp *ReceiptProcessor) GetPoints(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/receipts/")
	id = strings.TrimSuffix(id, "/points")

	rp.mux.Lock()
	points, exists := rp.data[id]
	rp.mux.Unlock()

	if !exists {
		http.Error(w, "No receipt found for that ID.", http.StatusNotFound)
		return
	}

	response := map[string]int{"points": points}
	json.NewEncoder(w).Encode(response)
}

func (rp *ReceiptProcessor) calculatePoints(receipt Receipt) int {
	points := 0

	// Rule: One point for every alphanumeric character in the retailer name
	points += len(regexp.MustCompile(`[a-zA-Z0-9]`).FindAllString(receipt.Retailer, -1))

	// Rule: 50 points if the total is a round dollar amount
	total, _ := strconv.ParseFloat(receipt.Total, 64)
	if math.Mod(total, 1.0) == 0 {
		points += 50
	}

	// Rule: 25 points if the total is a multiple of 0.25
	if math.Mod(total, 0.25) == 0 {
		points += 25
	}

	// Rule: 5 points for every two items
	points += (len(receipt.Items) / 2) * 5

	// Rule: Points for items with description length a multiple of 3
	for _, item := range receipt.Items {
		desc := strings.TrimSpace(item.ShortDescription)
		if len(desc)%3 == 0 {
			price, _ := strconv.ParseFloat(item.Price, 64)
			points += int(math.Ceil(price * 0.2))
		}
	}

	// Rule: 6 points if the day in the purchase date is odd
	day, _ := strconv.Atoi(strings.Split(receipt.PurchaseDate, "-")[2])
	if day%2 != 0 {
		points += 6
	}

	// Rule: 10 points if the time of purchase is between 2:00pm and 4:00pm
	parts := strings.Split(receipt.PurchaseTime, ":")
	hour, _ := strconv.Atoi(parts[0])
	if hour == 14 {
		points += 10
	}

	return points
}

func main() {
	rp := NewReceiptProcessor()

	http.HandleFunc("/receipts/process", rp.ProcessReceipt)
	http.HandleFunc("/receipts/", rp.GetPoints)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
