package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type SQLNode struct {
	Type     string
	Value    string
	Left     string
	Operator string
	Right    string
}

type Table struct {
	Name    string
	Headers []string
	Rows    [][]string
}

var printAST = flag.Bool("ast", false, "Print the Abstract Syntax Tree")

func main() {
	// Define the flag

	// Parse the flags
	flag.Parse()

	// Check if we have the correct number of arguments
	if flag.NArg() != 1 {
		fmt.Println("Usage: ./gosql [-ast] \"SQL QUERY\"")
		fmt.Println("SELECT, FROM, WHERE, and LIMIT are supported.")
		return
	}

	sqlQuery := flag.Arg(0)

	// Parse the SQL query
	ast, err := parseSQL(sqlQuery)
	if err != nil {
		fmt.Printf("Error parsing SQL: %v\n", err)
		return
	}

	// If the -ast flag is set, print the AST
	if *printAST {
		fmt.Println("Abstract Syntax Tree:")
		for _, node := range ast {
			fmt.Printf("%+v\n", node)
		}
	}

	result, err := executeQuery(ast)
	if err != nil {
		fmt.Printf("Error executing query: %v\n", err)
		return
	}

	for _, row := range result {
		fmt.Println(strings.Join(row, ", "))
	}
}

func executeQuery(ast []SQLNode) ([][]string, error) {
	var table *Table
	var limit int = -1
	var selectedColumns []string
	var whereClause *SQLNode

	for _, node := range ast {
		switch node.Type {
		case "SELECT":
			if node.Value == "*" {
				selectedColumns = nil // nil means select all columns
			} else {
				selectedColumns = strings.Split(node.Value, ", ")
			}
		case "FROM":
			tableName := node.Value + ".csv"
			loadedTable, err := loadCSVTable(tableName)
			if err != nil {
				return nil, fmt.Errorf("file '%s.csv' not found", node.Value)
			}
			table = loadedTable
		case "WHERE":
			whereClause = &node
		case "LIMIT":
			var err error
			limit, err = strconv.Atoi(node.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid LIMIT value: %s", node.Value)
			}
			if limit < 0 {
				return nil, fmt.Errorf("LIMIT value must be non-negative")
			}
		}
	}

	if table == nil {
		return nil, fmt.Errorf("no FROM clause found or table could not be loaded")
	}

	var columnIndices []int
	if selectedColumns == nil {
		// Select all columns
		columnIndices = make([]int, len(table.Headers))
		for i := range columnIndices {
			columnIndices[i] = i
		}
	} else {
		// Select specific columns
		for _, col := range selectedColumns {
			found := false
			for i, header := range table.Headers {
				if col == header {
					columnIndices = append(columnIndices, i)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("column %s not found in table", col)
			}
		}
	}

	// Prepare result with selected columns
	result := make([][]string, 0, len(table.Rows)+1)
	headerRow := make([]string, len(columnIndices))
	for i, idx := range columnIndices {
		headerRow[i] = table.Headers[idx]
	}
	result = append(result, headerRow)

	// Add data rows with selected columns
	for _, row := range table.Rows {
		if whereClause != nil {
			match, err := evaluateWhereClause(whereClause, table.Headers, row)
			if err != nil {
				return nil, err
			}
			if !match {
				continue
			}
		}

		if limit != -1 && len(result)-1 >= limit {
			break
		}
		newRow := make([]string, len(columnIndices))
		for i, idx := range columnIndices {
			newRow[i] = row[idx]
		}
		result = append(result, newRow)
	}

	return result, nil
}

func evaluateWhereClause(node *SQLNode, headers []string, row []string) (bool, error) {
	if node.Operator != "=" {
		return false, fmt.Errorf("only equality (=) operator is supported in WHERE clause")
	}

	leftIndex := -1
	for i, header := range headers {
		if header == node.Left {
			leftIndex = i
			break
		}
	}
	if leftIndex == -1 {
		return false, fmt.Errorf("column %s not found in table", node.Left)
	}

	leftValue := row[leftIndex]
	return leftValue == node.Right, nil
}

func loadCSVTable(filename string) (*Table, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	// Uppercase all headers
	for i, header := range records[0] {
		records[0][i] = strings.ToUpper(header)
	}

	return &Table{
		Name:    strings.TrimSuffix(filename, ".csv"),
		Headers: records[0],
		Rows:    records[1:],
	}, nil
}

func parseSQL(sql string) ([]SQLNode, error) {
	tokens := tokenize(sql)
	ast := []SQLNode{}

	for i := 0; i < len(tokens); {
		token := strings.ToUpper(tokens[i])
		switch token {
		case "SELECT":
			node := SQLNode{Type: token}
			i++
			values := []string{}
			for i < len(tokens) && strings.ToUpper(tokens[i]) != "FROM" {
				if tokens[i] != "," {
					values = append(values, strings.ToUpper(tokens[i]))
				}
				i++
			}
			node.Value = strings.Join(values, ", ")
			ast = append(ast, node)
		case "FROM":
			node := SQLNode{Type: token}
			i++
			values := []string{}
			for i < len(tokens) && !isKeyword(tokens[i]) {
				values = append(values, tokens[i])
				i++
			}
			node.Value = strings.Join(values, " ")
			ast = append(ast, node)
		case "WHERE":
			node := SQLNode{Type: token}
			i++
			if i+2 < len(tokens) {
				node.Left = strings.ToUpper(tokens[i])
				node.Operator = tokens[i+1]
				node.Right = tokens[i+2]
				node.Right = strings.Trim(node.Right, "'")
				i += 3
			}
			ast = append(ast, node)
		case "LIMIT":
			node := SQLNode{Type: token}
			i++
			if i < len(tokens) {
				node.Value = tokens[i]
				i++
			} else {
				return nil, fmt.Errorf("LIMIT clause requires a value")
			}
			ast = append(ast, node)
		default:
			return nil, fmt.Errorf("unexpected token: %s", token)
		}
	}

	return ast, nil
}

func tokenize(sql string) []string {
	var tokens []string
	var currentToken strings.Builder
	inQuotes := false

	for _, char := range sql {
		switch {
		case char == '\'':
			inQuotes = !inQuotes
			currentToken.WriteRune(char)
		case char == ' ' && !inQuotes:
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
		case char == ',' && !inQuotes:
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			tokens = append(tokens, ",")
		case char == '=' && !inQuotes:
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			tokens = append(tokens, "=")
		default:
			currentToken.WriteRune(char)
		}
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

func isKeyword(token string) bool {
	keywords := []string{"SELECT", "FROM", "WHERE", "LIMIT"}
	for _, keyword := range keywords {
		if strings.ToUpper(token) == keyword {
			return true
		}
	}
	return false
}
