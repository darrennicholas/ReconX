package reconciliation

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxImportBytes = 10 << 20
	MaxImportRows  = 50000
)

var ErrCSVInvalid = errors.New("csv file is invalid")

type ImportRowError struct {
	Line    int    `json:"line"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ParsedImport struct {
	Transactions []ImportedTransaction `json:"-"`
	Errors       []ImportRowError      `json:"errors"`
}

type ImportedTransaction struct {
	ReferenceRaw       string
	ReferenceCanonical string
	Date               time.Time
	Direction          string
	Amount             int64
	Currency           string
	RawRow             string
}

func ParseImportCSV(data []byte, periodFrom, periodTo time.Time) (ParsedImport, error) {
	if len(data) == 0 || len(data) > MaxImportBytes || !utf8.Valid(data) {
		return ParsedImport{}, ErrCSVInvalid
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = false
	header, err := reader.Read()
	if err != nil {
		return ParsedImport{}, ErrCSVInvalid
	}
	indices := make(map[string]int, len(header))
	for index, name := range header {
		name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if _, exists := indices[name]; exists {
			return ParsedImport{}, fmt.Errorf("%w: duplicate header %s", ErrCSVInvalid, name)
		}
		indices[name] = index
	}
	for _, required := range []string{"reference_id", "date", "direction", "amount", "currency"} {
		if _, exists := indices[required]; !exists {
			return ParsedImport{}, fmt.Errorf("%w: missing header %s", ErrCSVInvalid, required)
		}
	}

	result := ParsedImport{Transactions: make([]ImportedTransaction, 0)}
	seen := make(map[string]int)
	line := 1
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		line++
		if line-1 > MaxImportRows {
			return result, fmt.Errorf("%w: row limit exceeded", ErrCSVInvalid)
		}
		if readErr != nil {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "row", Message: "CSV row format is invalid"})
			continue
		}
		if isBlankRow(row) {
			continue
		}
		if len(row) != len(header) {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "row", Message: "Column count does not match header"})
			continue
		}
		value := func(key string) string { return strings.TrimSpace(row[indices[key]]) }
		reference := value("reference_id")
		canonical := strings.ToUpper(reference)
		date, dateErr := time.Parse("2006-01-02", value("date"))
		direction := strings.ToUpper(value("direction"))
		amount, amountErr := strconv.ParseInt(value("amount"), 10, 64)
		currency := strings.ToUpper(value("currency"))
		rowInvalid := false
		if reference == "" {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "reference_id", Message: "Reference cannot be empty"})
			rowInvalid = true
		}
		if reference != "" {
			if firstLine, exists := seen[canonical]; exists {
				result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "reference_id", Message: fmt.Sprintf("Duplicate reference (first seen on line %d)", firstLine)})
				rowInvalid = true
			} else {
				seen[canonical] = line
			}
		}
		if dateErr != nil || date.Format("2006-01-02") != value("date") {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "date", Message: "Date must use YYYY-MM-DD"})
			rowInvalid = true
		} else if date.Before(periodFrom) || date.After(periodTo) {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "date", Message: "Date falls outside the batch period"})
			rowInvalid = true
		}
		if direction != "CREDIT" && direction != "DEBIT" {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "direction", Message: "Direction must be CREDIT or DEBIT"})
			rowInvalid = true
		}
		if amountErr != nil || amount <= 0 {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "amount", Message: "Amount must be a positive integer in rupiah"})
			rowInvalid = true
		}
		if currency != "IDR" {
			result.Errors = append(result.Errors, ImportRowError{Line: line, Field: "currency", Message: "Currency must be IDR"})
			rowInvalid = true
		}
		if rowInvalid {
			continue
		}
		raw, _ := json.Marshal(row)
		result.Transactions = append(result.Transactions, ImportedTransaction{ReferenceRaw: reference, ReferenceCanonical: canonical, Date: date, Direction: direction, Amount: amount, Currency: currency, RawRow: string(raw)})
	}
	if len(result.Transactions)+len(result.Errors) == 0 {
		result.Errors = append(result.Errors, ImportRowError{Line: 1, Field: "file", Message: "CSV must contain at least one transaction"})
	}
	return result, nil
}

func isBlankRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func ErrorCSV(errors []ImportRowError) []byte {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	_ = writer.Write([]string{"line", "field", "message"})
	for _, row := range errors {
		_ = writer.Write([]string{strconv.Itoa(row.Line), row.Field, row.Message})
	}
	writer.Flush()
	return output.Bytes()
}
