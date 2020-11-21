package prolog

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

type Parser struct {
	lexer     *Lexer
	current   Token
	operators operators
}

func NewParser(input string, operators operators) *Parser {
	p := Parser{
		lexer:     NewLexer(input),
		operators: operators,
	}
	p.current = p.lexer.Next()
	return &p
}

func (p *Parser) accept(k TokenKind, vals ...string) (string, error) {
	v, err := p.expect(k, vals...)
	if err != nil {
		return "", err
	}
	p.current = p.lexer.Next()
	return v, nil
}

func (p *Parser) acceptOp(min int) (*operator, error) {
	for _, op := range p.operators {
		// we don't consume the current token here since the following check might tell it's not the operator we're looking for.
		if _, err := p.expect(TokenAtom, string(op.Name)); err != nil {
			continue
		}

		l, _ := op.bindingPowers()
		if l < min {
			continue
		}

		// checks are ok. consume the current token.
		p.current = p.lexer.Next()

		return &op, nil
	}
	return nil, errors.New("no op")
}

func (p *Parser) expect(k TokenKind, vals ...string) (string, error) {
	if p.current.Kind != k {
		return "", &unexpectedToken{
			ExpectedKind: k,
			ExpectedVals: vals,
			Actual:       p.current,
		}
	}

	if len(vals) > 0 {
		for _, v := range vals {
			if v == p.current.Val {
				return v, nil
			}
		}
		return "", &unexpectedToken{
			ExpectedKind: k,
			ExpectedVals: vals,
			Actual:       p.current,
		}
	}

	return p.current.Val, nil
}

func (p *Parser) Program() ([]Term, error) {
	var ret []Term
	for {
		if _, err := p.accept(TokenEOS); err == nil {
			return ret, nil
		}

		c, err := p.Clause()
		if err != nil {
			return nil, err
		}
		ret = append(ret, c)
	}
}

func (p *Parser) Clause() (Term, error) {
	t, err := p.Term()
	if err != nil {
		return nil, err
	}

	if _, err := p.accept(TokenSeparator, "."); err != nil {
		return nil, fmt.Errorf("clause: %w", err)
	}

	return t, nil
}

func (p *Parser) Term() (Term, error) {
	if _, err := p.accept(TokenSeparator, "("); err == nil {
		t, err := p.Term()
		if err != nil {
			return nil, fmt.Errorf("term: %w", err)
		}
		if _, err := p.accept(TokenSeparator, ")"); err != nil {
			return nil, fmt.Errorf("term: %w", err)
		}
		return t, nil
	}

	return p.expr(1)
}

// based on Pratt parser explained in this article: https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html
func (p *Parser) expr(min int) (Term, error) {
	if t, err := p.prefixUnary(); err == nil {
		return t, nil
	}

	lhs, err := p.expr0()
	if err != nil {
		return nil, err
	}

	for {
		op, err := p.acceptOp(min)
		if err != nil {
			break
		}

		_, r := op.bindingPowers()
		rhs, err := p.expr(r)
		if err != nil {
			return nil, err
		}

		lhs = &Compound{
			Functor: op.Name,
			Args:    []Term{lhs, rhs},
		}
	}

	return lhs, nil
}

func (p *Parser) prefixUnary() (Term, error) {
	for _, op := range p.operators {
		l, r := op.bindingPowers()
		if l != 0 {
			continue
		}

		if _, err := p.accept(TokenAtom, string(op.Name)); err != nil {
			continue
		}

		x, err := p.expr(r)
		if err != nil {
			return nil, err
		}

		return &Compound{
			Functor: op.Name,
			Args:    []Term{x},
		}, nil
	}

	return nil, errors.New("not unary")
}

func (p *Parser) expr0() (Term, error) {
	a, err := p.accept(TokenAtom)
	if err != nil {
		i, err := p.accept(TokenInteger)
		if err == nil {
			n, _ := strconv.Atoi(i)
			return Integer(n), nil
		}

		v, err := p.accept(TokenVariable)
		if err != nil {
			return nil, fmt.Errorf("expr0: %w", err)
		}
		return &Variable{
			Name: v,
		}, nil
	}

	if _, err := p.accept(TokenSeparator, "("); err != nil {
		return Atom(a), nil
	}

	var args []Term
	for {
		t, err := p.Term()
		if err != nil {
			return nil, err
		}
		args = append(args, t)

		sep, err := p.accept(TokenSeparator, ",", ")")
		if err != nil {
			return nil, fmt.Errorf("expr0: %w", err)
		}
		if sep == ")" {
			break
		}
	}

	return &Compound{Functor: Atom(a), Args: args}, nil
}

type operators []operator

func (os operators) Len() int {
	return len(os)
}

func (os operators) Less(i, j int) bool {
	return os[i].Precedence > os[j].Precedence
}

func (os operators) Swap(i, j int) {
	os[i], os[j] = os[j], os[i]
}

func (os operators) atMost(p int) operators {
	i := sort.Search(len(os), func(i int) bool { return int(os[i].Precedence) <= p })
	if i == len(os) {
		return nil // not found
	}
	return os[i:]
}

type operator struct {
	Precedence Integer // 1 ~ 1200
	Type       Atom
	Name       Atom
}

func (o *operator) bindingPowers() (int, int) {
	bp := 1201 - int(o.Precedence) // 1 ~ 1200
	switch o.Type {
	case "xf":
		return bp + 1, 0
	case "yf":
		return bp, -1
	case "xfx":
		return bp + 1, bp + 1
	case "xfy":
		return bp + 1, bp
	case "yfx":
		return bp, bp + 1
	case "fx":
		return 0, bp + 1
	case "fy":
		return 0, bp
	default:
		return 0, 0
	}
}

type unexpectedToken struct {
	ExpectedKind TokenKind
	ExpectedVals []string
	Actual       Token
}

func (e *unexpectedToken) Error() string {
	return fmt.Sprintf("expected: <%s %s>, actual: %s", e.ExpectedKind, e.ExpectedVals, e.Actual)
}