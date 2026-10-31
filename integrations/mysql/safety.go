package mysql

import (
	"fmt"
	"strings"
)

type sqlToken struct {
	text  string
	kind  byte
	start int
	end   int
}

var allowedFunctions = map[string]bool{
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"COALESCE": true, "IFNULL": true, "NULLIF": true, "IF": true,
	"CAST": true, "CONVERT": true, "CONCAT": true, "CONCAT_WS": true,
	"LOWER": true, "UPPER": true, "LENGTH": true, "CHAR_LENGTH": true,
	"SUBSTRING": true, "SUBSTR": true, "TRIM": true, "LTRIM": true, "RTRIM": true,
	"ROUND": true, "FLOOR": true, "CEIL": true, "ABS": true, "MOD": true,
	"DATE": true, "YEAR": true, "MONTH": true, "DAY": true, "DATE_FORMAT": true,
	"DATEDIFF": true, "TIMESTAMPDIFF": true, "NOW": true, "CURRENT_DATE": true,
	"JSON_EXTRACT": true, "JSON_UNQUOTE": true, "JSON_LENGTH": true,
	"ROW_NUMBER": true, "RANK": true, "DENSE_RANK": true, "LAG": true, "LEAD": true,
	"VERSION": true, "DATABASE": true, "CURRENT_USER": true,
}

var forbiddenWords = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true,
	"CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true,
	"GRANT": true, "REVOKE": true, "CALL": true, "DO": true, "HANDLER": true,
	"LOAD": true, "INTO": true, "OUTFILE": true, "DUMPFILE": true,
	"SET": true, "RESET": true, "KILL": true, "SHUTDOWN": true,
	"LOCK": true, "UNLOCK": true, "FOR": true, "PROCEDURE": true,
	"ANALYZE": true, "BENCHMARK": true, "SLEEP": true,
}

func tokenize(query string) ([]sqlToken, error) {
	if len(query) > 64<<10 {
		return nil, fmt.Errorf("mysql: SQL exceeds 64 KiB")
	}
	var tokens []sqlToken
	for offset := 0; offset < len(query); {
		char := query[offset]
		if strings.ContainsRune(" \t\r\n", rune(char)) {
			offset++
			continue
		}
		if char == ';' || char == '#' || char == '@' || char == '\\' || char == '"' || char > 126 || char < 32 || strings.HasPrefix(query[offset:], "/*") || strings.HasPrefix(query[offset:], "--") || strings.HasPrefix(query[offset:], ":=") {
			return nil, fmt.Errorf("mysql: comments, multiple statements, variables and ambiguous SQL syntax are not supported")
		}
		if char == '\'' || char == '`' {
			start := offset
			offset++
			closed := false
			for offset < len(query) {
				if query[offset] == '\\' {
					return nil, fmt.Errorf("mysql: backslash escapes are not supported")
				}
				if query[offset] == char {
					offset++
					if offset < len(query) && query[offset] == char {
						offset++
						continue
					}
					closed = true
					break
				}
				offset++
			}
			if !closed {
				return nil, fmt.Errorf("mysql: unterminated SQL quote")
			}
			tokens = append(tokens, sqlToken{text: query[start:offset], kind: char, start: start, end: offset})
			continue
		}
		if isWord(char) {
			start := offset
			for offset < len(query) && isWord(query[offset]) {
				offset++
			}
			tokens = append(tokens, sqlToken{text: strings.ToUpper(query[start:offset]), kind: 'w', start: start, end: offset})
			continue
		}
		if !strings.ContainsRune("(),.*+-/%=<>!?|&^~:", rune(char)) {
			return nil, fmt.Errorf("mysql: unsupported SQL character")
		}
		tokens = append(tokens, sqlToken{text: string(char), kind: 'p', start: offset, end: offset + 1})
		offset++
	}
	return tokens, nil
}

func isWord(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '$'
}

func validateQuery(query string) error {
	tokens, err := tokenize(query)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("mysql: query is required")
	}
	if tokens[0].text == "SHOW" {
		return validateShow(tokens)
	}
	if tokens[0].text == "EXPLAIN" {
		tokens = tokens[1:]
		if len(tokens) >= 3 && tokens[0].text == "FORMAT" && tokens[1].text == "=" && tokens[2].text == "JSON" {
			tokens = tokens[3:]
		}
	}
	if len(tokens) == 0 || tokens[0].text != "SELECT" || tokens[0].kind != 'w' {
		return fmt.Errorf("mysql: only SELECT, supported SHOW, and EXPLAIN SELECT are allowed")
	}
	for index, token := range tokens {
		if token.kind == 'w' && forbiddenWords[token.text] {
			return fmt.Errorf("mysql: unsafe SQL statement class or clause")
		}
		if index+1 >= len(tokens) || tokens[index+1].text != "(" || token.kind != 'w' && token.kind != '`' {
			continue
		}
		if token.kind == '`' || index > 0 && tokens[index-1].text == "." {
			return fmt.Errorf("mysql: arbitrary or qualified routines are not allowed")
		}
		if allowedFunctions[token.text] {
			if token.end != tokens[index+1].start {
				return fmt.Errorf("mysql: built-in function names must directly precede parentheses")
			}
			continue
		}
		if index > 0 && tokens[index-1].text == "AS" {
			continue
		}
		if token.text == "USING" {
			continue
		}
		if token.text == "BY" && index > 0 && (tokens[index-1].text == "ORDER" || tokens[index-1].text == "GROUP") {
			continue
		}
		switch token.text {
		case "IN", "EXISTS", "OVER", "AS", "FROM", "JOIN", "WHERE", "AND", "OR", "NOT", "ON", "HAVING", "SELECT", "CASE", "WHEN", "THEN", "ELSE", "END",
			"DISTINCT", "DISTINCTROW", "ALL", "LIKE", "REGEXP", "RLIKE", "BETWEEN", "XOR":
			continue
		}
		return fmt.Errorf("mysql: function is not in the supported read-only built-in allowlist")
	}
	return nil
}

func validateShow(tokens []sqlToken) error {
	words := make([]string, len(tokens))
	for index, token := range tokens {
		words[index] = token.text
	}
	if len(words) == 2 && words[1] == "DATABASES" {
		return nil
	}
	start := 1
	if len(words) > start && words[start] == "FULL" {
		start++
	}
	if len(words) > start && words[start] == "TABLES" {
		if len(words) == start+1 || len(words) == start+3 && words[start+1] == "FROM" && sqlIdentifier(tokens[start+2]) {
			return nil
		}
	}
	if len(words) >= 4 && words[1] == "CREATE" && (words[2] == "TABLE" || words[2] == "VIEW") {
		if len(words) == 4 && sqlIdentifier(tokens[3]) || len(words) == 6 && sqlIdentifier(tokens[3]) && words[4] == "." && sqlIdentifier(tokens[5]) {
			return nil
		}
	}
	return fmt.Errorf("mysql: supported SHOW statements are DATABASES, [FULL] TABLES [FROM database], and CREATE TABLE/VIEW")
}

func sqlIdentifier(token sqlToken) bool {
	return token.kind == 'w' || token.kind == '`'
}
