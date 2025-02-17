package term

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sync/atomic"
)

// Variable is a prolog variable.
type Variable string

var varCounter uint64

func NewVariable() Variable {
	atomic.AddUint64(&varCounter, 1)
	return Variable(fmt.Sprintf("_%d", varCounter))
}

var anonVarPattern = regexp.MustCompile(`\A_\d+\z`)

func (v Variable) Anonymous() bool {
	return anonVarPattern.MatchString(string(v))
}

func (v Variable) String() string {
	var buf bytes.Buffer
	_ = v.WriteTerm(&buf, DefaultWriteTermOptions, nil)
	return buf.String()
}

// WriteTerm writes the variable into w.
func (v Variable) WriteTerm(w io.Writer, opts WriteTermOptions, env *Env) error {
	ref, ok := env.Lookup(v)
	if ok && opts.Descriptive {
		if v != "" {
			if _, err := fmt.Fprintf(w, "%s = ", v); err != nil {
				return err
			}
		}
		return ref.WriteTerm(w, opts, env)
	}
	_, err := fmt.Fprint(w, string(v))
	return err
}

// Unify unifies the variable with t.
func (v Variable) Unify(t Interface, occursCheck bool, env *Env) (*Env, bool) {
	r, t := env.Resolve(v), env.Resolve(t)
	v, ok := r.(Variable)
	if !ok {
		return r.Unify(t, occursCheck, env)
	}
	switch {
	case v == t:
		return env, true
	case occursCheck && Contains(t, v, env):
		return env, false
	default:
		return env.Bind(v, t), true
	}
}
