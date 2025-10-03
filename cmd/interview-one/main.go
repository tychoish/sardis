package main

import (
	"fmt"
	"strconv"
)

type Operator string

const (
	OperatorAddition       Operator = "+"
	OperatorSubtration              = "-"
	OperatorDivision                = "/"
	OperatorMultiplication          = "*"
	OperatorEqual                   = "="
)

const (
	Operators = "+-/*"
	Numbers   = "1234567890"
)

// -,+,*,/

// inputs: "1+21", "5 *94" "1 + 4 * 4 + 4"

type Operation struct {
	Left     int
	Right    *Operation
	Operator Operator
	Value    int
}

func evaluate(op *Operation) int {
	head := op

	for {
		switch op.Operator {
		case OperatorEqual:
			continue
		case OperatorMultiplication:
			op.Value = op.Left * op.Right.Left
			op.Right = op.Right.Right
		}
		if op.Right == nil {
			break
		}
		op = op.Right
	}
	op = head
	for {
		switch op.Operator {
		case OperatorEqual:
			continue
		case OperatorDivision:
			op.Value = op.Left / op.Right.Left
			op.Right = op.Right.Right
		}
		if op.Right == nil {
			break
		}
		op = op.Right
	}

	op = head
	for {
		switch op.Operator {
		case OperatorEqual:
			continue
		case OperatorAddition:
			op.Value = op.Left + op.Right.Left
			op.Right = op.Right.Right
		}
		if op.Right == nil {
			break
		}
		op = op.Right
	}

	op = head
	for {
		switch op.Operator {
		case OperatorEqual:
			continue
		case OperatorSubtration:
			op.Value = op.Left - op.Right.Left
			op.Right = op.Right.Right
		}
		if op.Right == nil {
			break
		}
		op = op.Right
	}

	op = head
	for {
		switch op.Operator {
		case OperatorEqual:
			continue
		case OperatorMultiplication:
			op.Value = op.Left * op.Right.Left
			op.Right = op.Right.Right
		}
		if op.Right == nil {
			break
		}
	}

	op = head
	switch op.Operator {
	case OperatorEqual:
		return op.Value
	case OperatorSubtration:
		return op.Left - op.Value
	case OperatorAddition:
		return op.Left + op.Value
	case OperatorDivision:
		return op.Left / op.Value
	case OperatorMultiplication:
		return op.Left * op.Value
	default:
		panic("shouldn't be possible")
	}
}

func parse(input string) Operation {
	output := Operation{}
	var previous string
	for idx, char := range input {
		shouldParsePrevious := false
		switch char {
		case ' ':
		// pass
		case '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
			previous = fmt.Sprint(previous, char)
		case '/':
			shouldParsePrevious = true
			output.Operator = OperatorDivision
		case '*':
			shouldParsePrevious = true
			output.Operator = OperatorMultiplication
		case '+':
			shouldParsePrevious = true
			output.Operator = OperatorAddition
		case '-':
			shouldParsePrevious = true
			output.Operator = OperatorSubtration
		}

		if shouldParsePrevious {
			val, err := strconv.Atoi(previous)
			if err != nil {
				panic(err)
			}

			output.Left = val
			if idx == len(input)-1 {
				output.Right = &Operation{Value: 0, Operator: OperatorEqual}
				return output
			}

			next := parse(input[idx+1:])
			output.Right = &next
		}
	}

	// ??
	return output
}

func main() {
	op := parse("1+2")
	val := evaluate(&op)
	fmt.Println(val)
}
