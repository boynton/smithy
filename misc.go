package smithy

import (
	"bytes"
	"fmt"
	"strings"
)

var Verbose bool

func Debug(args ...interface{}) {
	if Verbose {
		max := len(args) - 1
		for i := 0; i < max; i++ {
			fmt.Print(str(args[i]))
		}
		fmt.Println(str(args[max]))
	}
}

func str(arg interface{}) string {
	return fmt.Sprintf("%v", arg)
}

func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}

func Uncapitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[0:1]) + s[1:]
}

func IsSymbolChar(ch rune, first bool) bool {
	if IsLetter(ch) {
		return true
	}
	if first {
		return false
	}
	return IsDigit(ch) || ch == '_'
}

/*
func IsSymbol(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !IsSymbolChar(c, i == 0) {
			return false
		}
	}
	return true
}
*/

func IsWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func IsDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func IsLetter(ch rune) bool {
	return IsUppercaseLetter(ch) || IsLowercaseLetter(ch)
}

func IsUppercaseLetter(ch rune) bool {
	return ch >= 'A' && ch <= 'Z'
}

func IsLowercaseLetter(ch rune) bool {
	return ch >= 'a' && ch <= 'z'
}

func FormatComment(indent, prefix, comment string, maxcol int, extraPad bool) string {
	left := len(indent)
	if maxcol <= left && strings.Index(comment, "\n") < 0 {
		return indent + prefix + comment + "\n"
	}
	tabbytes := make([]byte, 0, left)
	for i := 0; i < left; i++ {
		tabbytes = append(tabbytes, ' ')
	}
	tab := string(tabbytes)
	prefixlen := len(prefix)
	if strings.Index(comment, "\n") >= 0 {
		lines := strings.Split(comment, "\n")
		result := ""
		for _, line := range lines {
			result = result + tab + prefix + line + "\n"
		}
		return result
	}
	var buf bytes.Buffer
	col := 0
	lines := 1
	tokens := strings.Split(comment, " ")
	for _, tok := range tokens {
		toklen := len(tok)
		if col+toklen >= maxcol {
			buf.WriteString("\n")
			lines++
			col = 0
		}
		if col == 0 {
			buf.WriteString(tab)
			buf.WriteString(prefix)
			buf.WriteString(tok)
			col = left + prefixlen + toklen
		} else {
			buf.WriteString(" ")
			buf.WriteString(tok)
			col += toklen + 1
		}
	}
	buf.WriteString("\n")
	emptyPrefix := strings.Trim(prefix, " ")
	//	pad := tab + emptyPrefix + "\n"
	pad := ""
	if extraPad {
		pad = tab + emptyPrefix + "\n"
	}
	return pad + buf.String() + pad
}

func TrimSpace(s string) string {
	return TrimLeftSpace(TrimRightSpace(s))
}

func TrimRightSpace(s string) string {
	return strings.TrimRight(s, " \t\n\v\f\r")
}

func TrimLeftSpace(s string) string {
	return strings.TrimLeft(s, " \t\n\v\f\r")
}
