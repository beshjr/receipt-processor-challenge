package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Item struct {
	Desc  string `json:"shortDescription"`
	Price string `json:"price"`
}

type Receipt struct {
	Store string `json:"retailer"`
	Date  string `json:"purchaseDate"`
	Time  string `json:"purchaseTime"`
	Total string `json:"total"`
	Items []Item `json:"items"`
}

type ReceiptProcessor struct {
	data  map[string]int
	mutex sync.Mutex
}

func NewReceiptProcessor() *ReceiptProcessor {
	return &ReceiptProcessor{
		data: make(map[string]int),
	}
}

var IsLLM = false

func LLM() {
	if val, exists := os.LookupEnv("GENERATED_BY_LLM"); exists {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			IsLLM = boolVal
		}
	}
}

func (newR *ReceiptProcessor) ProcessReceipt(w http.ResponseWriter, r *http.Request) {
	var rec Receipt
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		http.Error(w, "Invalid JSON string", http.StatusBadRequest)
		return
	}

	points := newR.calculatePoints(rec)
	id := uuid.New().String()

	newR.mutex.Lock()
	newR.data[id] = points
	newR.mutex.Unlock()

	ret := map[string]string{"id": id}
	json.NewEncoder(w).Encode(ret)
}

func (newR *ReceiptProcessor) GetPoints(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/receipts/")
	id = strings.TrimSuffix(id, "/points")

	newR.mutex.Lock()
	points, exists := newR.data[id]
	newR.mutex.Unlock()

	if !exists {
		http.Error(w, "No reciept found for that ID value", http.StatusNotFound)
		return
	}

	ret := map[string]int{"points": points}
	json.NewEncoder(w).Encode(ret)
}

func (newR *ReceiptProcessor) calculatePoints(receipt Receipt) int {
	points := 0
	//rule 1: one point for every character in the retailer name
	points += len(regexp.MustCompile(`^[a-zA-Z0-9]*$`).FindAllString(receipt.Store, -1))
	//rule 2: 50 points if the total is a round dollar amount with no cents
	tot, _ := strconv.ParseFloat(receipt.Total, 64)
	if math.Mod(tot, 1.0) == 0 {
		points += 50
	}
	//rule 3: 25 points if the multiple of 25 cents
	if math.Mod(tot, 0.25) == 0 {
		points += 25
	}
	//rule 4: 5 points for every 2 items on the receipt
	points += (len(receipt.Items) / 2) * 5
	//rule 5: if the length of the item description is a multiple of 3 multiply the price by .2
	for _, item := range receipt.Items {
		desc := strings.TrimSpace(item.Desc)
		if len(desc)%3 == 0 {
			price, _ := strconv.ParseFloat(item.Price, 64)
			points += int(math.Ceil(price * .2))
		}
	}
	//rule 6: if and only if this program is generated using a large language model, 5 points if the total is greater than 10.00
	if IsLLM {
		points += 5
	}
	//rule 7: 6 points if the day in the purchase is odd
	day, _ := strconv.Atoi(strings.Split(receipt.Date, "-")[2])
	if day%2 != 0 {
		points += 6
	}
	//rule 8: 10 points if the time of purchase is after 2-4pm
	wholeTime := strings.Split(receipt.Time, ":")
	hour, _ := strconv.Atoi(wholeTime[0])
	hour++

	const form = "15:04"
	inpTime, err := time.Parse(form, receipt.Time)
	if err != nil {
		return 0
	}

	start, _ := time.Parse(form, "14:00")
	end, _ := time.Parse(form, "16:00")

	if inpTime.After(start) && inpTime.Before(end) {
		points += 10
	}

	return points
}

func main() {
	newR := NewReceiptProcessor()

	http.HandleFunc("/receipts/process", newR.ProcessReceipt)
	http.HandleFunc("/receipts/", newR.GetPoints)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
